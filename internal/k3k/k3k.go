package k3k

import (
	"fmt"
	"github.com/ravan/so-k3k/internal/config"
	"github.com/ravan/stackstate-client/stackstate/api"
	"github.com/samber/lo"
	"log/slog"
	"strings"
)

type K3K struct {
	client   *api.Client
	clusters map[string]*Cluster
}

func New(conf *config.Configuration) *K3K {
	return &K3K{
		client:   api.NewClient(&conf.SuseObservability),
		clusters: map[string]*Cluster{},
	}
}

func (k *K3K) ConsolidateModel() ([]*Cluster, error) {
	chain := []func() error{k.loadClusterInfo, k.loadNodeInfo, k.loadSharedPodInfo}
	for _, f := range chain {
		if err := f(); err != nil {
			return nil, err
		}
	}
	return lo.Values(k.clusters), nil
}

func (k *K3K) GetClusters() map[string]*Cluster {
	return k.clusters
}

func (k *K3K) GetCluster(name string) *Cluster {
	return k.clusters[name]
}

func (k *K3K) loadSharedPodInfo() error {
	for _, nativeCluster := range k.clusters {
		sharedClusters := nativeCluster.GetSharedClusters()
		for _, sc := range sharedClusters {
			if err := k.processSharedPods(sc); err != nil {
				return err
			}
			if err := k.processVirtualNodePod(sc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (k *K3K) processVirtualNodePod(sc *Cluster) error {
	query := fmt.Sprintf(k3kVirtualNodePodQuery, sc.OwnerClusterName, sc.Namespace, sc.Name)
	slog.Debug("find virtual node pod", "query", query)
	podComponent, err := k.client.SnapShotTopologyQuery(query)
	if err != nil {
		return err
	}
	if len(podComponent) != 1 {
		slog.Warn("Expecting to find 1 pod associated with node", "node", sc.VirtualNode.NodeName, "count", len(podComponent))
	} else {
		pod := podComponent[0]
		identifier, _ := lo.Find(pod.Identifiers, func(identifier string) bool {
			return strings.Contains(identifier, sc.VirtualNode.NodeName)
		})
		sc.VirtualNode.PodName = pod.Name
		sc.VirtualNode.PodIdentifier = identifier
	}
	return nil
}

func (k *K3K) processSharedPods(sc *Cluster) error {
	query := fmt.Sprintf(k3kSharedPodsQuery, sc.OwnerClusterName, sc.Namespace, sc.Name, sc.Name)
	slog.Debug("find shared pods", "query", query)
	podComponents, err := k.client.SnapShotTopologyQuery(query)
	if err != nil {
		return err
	}

	pods := lo.Map(podComponents, func(item api.ViewComponent, index int) *Pod {
		identifier, found := lo.Find(item.Identifiers, func(identifier string) bool {
			return strings.HasSuffix(identifier, item.Name)
		})
		if !found {
			//should never happen
			panic(fmt.Sprintf("unable to find identifier with suffix '%s'", item.Name))
		}
		return &Pod{
			Name:        item.Name,
			ClusterName: k.LabelValue(ClusterName, item.Tags),
			Identifier:  identifier,
			Namespace:   k.LabelValue(Namespace, item.Tags),
		}
	})

	err = k.pairSharedPods(pods, sc)
	if err != nil {
		return err
	}
	return nil
}

func (k *K3K) pairSharedPods(pods []*Pod, shareCluster *Cluster) error {
	podsByCluster := lo.GroupBy(pods, func(item *Pod) string {
		return item.ClusterName
	})

	hostedPods := podsByCluster[shareCluster.OwnerClusterName]
	mappedPods := lo.FilterMap(podsByCluster[shareCluster.Name], func(pod *Pod, index int) (*Pod, bool) {
		hostPod, found := lo.Find(hostedPods, func(hostPod *Pod) bool {
			return strings.HasPrefix(hostPod.Name, pod.Name)
		})
		if !found {
			slog.Error("couldn't find hosted pod", "pod", pod.Name)
			return nil, false
		}
		pod.SharedPodName = hostPod.Name
		pod.SharedPodUID = hostPod.Identifier
		pod.OwnerClusterName = hostPod.ClusterName
		pod.OwnerNamespace = hostPod.Namespace
		return pod, true
	})
	shareCluster.SharedPods = mappedPods
	return nil
}

func (k *K3K) loadNodeInfo() error {
	for _, nativeCluster := range k.clusters {
		namespaces := strings.Join(nativeCluster.getNestedClusterNames(), ", ")
		nodeQuery := fmt.Sprintf(k3kServerNodesQuery, namespaces)
		slog.Debug("loading node info", "query", nodeQuery)
		nodes, err := k.client.SnapShotTopologyQuery(nodeQuery)
		if err != nil {
			return err
		}
		lo.ForEach(nodes, func(node api.ViewComponent, index int) {
			clusterName := k.LabelValue(ClusterName, node.Tags)
			cluster := nativeCluster.GetCluster(clusterName)
			if cluster == nil {
				slog.Error("cluster not found for node.", "cluster", clusterName, "node", node.Name)
			} else {
				if k.LabelExists("type:virtual-kubelet", node.Tags) {
					cluster.Shared = true
					cluster.VirtualNode.NodeName = node.Name
					cluster.VirtualNode.NodeIdentifier = fmt.Sprintf("urn:kubernetes:/%s:node/%s", clusterName, node.Name)
					lo.ForEach(lo.Values(cluster.Servers), func(server *Server, index int) {
						server.NodeIdentifier = cluster.VirtualNode.NodeIdentifier
						server.NodeName = node.Name
					})

				} else if server, ok := cluster.Servers[node.Name]; ok {
					server.NodeIdentifier = fmt.Sprintf("urn:kubernetes:/%s:node/%s", clusterName, node.Name)
					server.NodeName = node.Name
				} else {
					slog.Error("server not in cluster found for node.", "cluster", clusterName, "node", node.Name, "server", node.Name)
				}
			}
		})
	}
	return nil
}

func (k *K3K) loadClusterInfo() error {
	//find all k3k server pods
	comps, err := k.client.SnapShotTopologyQuery(k3kServerQuery)
	if err != nil {
		return err

	}
	//make sure we don't have false positives by checking if they start with k3k.
	servers := lo.FilterMap(comps, func(item api.ViewComponent, index int) (*Server, bool) {
		if strings.HasPrefix(item.Name, k3kPrefix) {
			return &Server{
				Name:                  item.Name,
				ClusterName:           k.LabelValue("cluster", item.Tags),
				OwnerClusterNamespace: k.LabelValue(Namespace, item.Tags),
				OwnerClusterName:      k.LabelValue(ClusterName, item.Tags),
				Identifiers:           item.Identifiers,
			}, true
		}
		return nil, false
	})

	//create clusters for servers
	vClusters := make(map[string]*Cluster)
	lo.ForEach(servers, func(item *Server, index int) {
		if cluster, ok := vClusters[item.ClusterName]; ok {
			//exists
			cluster.Servers[item.Name] = item
		} else {
			//new
			vClusters[item.ClusterName] = &Cluster{
				Name:             item.ClusterName,
				Namespace:        item.OwnerClusterNamespace,
				OwnerClusterName: item.OwnerClusterName,
				Virtual:          true,
				VirtualNode:      &VirtualNode{},
				Shared:           false,
				Nested:           make(map[string]*Cluster),
				Servers:          make(map[string]*Server),
			}
			vClusters[item.ClusterName].Servers[item.Name] = item
		}
	})

	//create native clusters and organize nested clusters in their hierarchy
	nativeClusters := make(map[string]*Cluster)
	lo.ForEach(lo.Values(vClusters), func(item *Cluster, index int) {
		if owner, ok := vClusters[item.OwnerClusterName]; ok {
			//virtual cluster
			owner.add(item)
		} else {
			//native cluster
			nativeClusters[item.OwnerClusterName] = &Cluster{
				Name:   item.OwnerClusterName,
				Nested: make(map[string]*Cluster),
			}
			nativeClusters[item.OwnerClusterName].add(item)
		}
	})
	k.clusters = nativeClusters
	return nil
}

func (k *K3K) LabelExists(name string, labels []string) bool {
	found := lo.FindOrElse(labels, "", func(item string) bool {
		return item == name
	})
	return found != ""
}

func (k *K3K) LabelValue(name string, labels []string) string {
	// labels have the format "<key>:<value>" or "key"
	for _, label := range labels {
		if strings.HasPrefix(label, fmt.Sprintf("%s:", name)) {
			labelAndValue := strings.Split(label, ":")
			if len(labelAndValue) == 2 {
				return labelAndValue[1]
			}
		}
	}
	return ""
}
