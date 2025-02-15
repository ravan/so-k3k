package k3k

import (
	"fmt"
	"github.com/samber/lo"
	"strings"
)

const (
	k3kServerQuery      = "type = 'pod' and label = 'role:server'"
	k3kServerNodesQuery = "type = 'node' AND label in (%s)"
	k3kSharedPodsQuery  = "" +
		"(type = 'pod' and label = 'cluster-name:%s' and label = 'namespace:%s' and label = 'k3k.io/clusterName:%s')" +
		" OR (type = 'pod' and label = 'cluster-name:%s')"
	k3kVirtualNodePodQuery = "" +
		"type = 'pod' and label = 'type:agent' and label = 'mode:shared' and label = 'cluster-name:%s' and " +
		"label = 'namespace:%s' and label = 'cluster:%s'"
	k3kPrefix = "k3k-"

	ClusterName = "cluster-name"
	Namespace   = "namespace"
)

type Pod struct {
	Name             string
	ClusterName      string
	Namespace        string
	Identifier       string
	SharedPodName    string
	SharedPodUID     string
	OwnerClusterName string
	OwnerNamespace   string
}

type Server struct {
	Name                  string
	ClusterName           string
	OwnerClusterName      string
	OwnerClusterNamespace string
	Identifiers           []string
	NodeIdentifier        string
	NodeName              string
}

func (s Server) MustGetIdentifier() string {
	identifier, found := lo.Find(s.Identifiers, func(identifier string) bool {
		return strings.HasSuffix(identifier, s.Name)
	})
	if !found {
		//should never happen
		panic(fmt.Sprintf("unable to find identifier with suffix '%s'", s.Name))
	}
	return identifier
}

type VirtualNode struct {
	NodeIdentifier string
	NodeName       string
	PodIdentifier  string
	PodName        string
}
type Cluster struct {
	Name             string
	Namespace        string
	OwnerClusterName string
	OwnerCluster     *Cluster
	Virtual          bool
	Shared           bool
	VirtualNode      *VirtualNode
	Nested           map[string]*Cluster
	Servers          map[string]*Server
	SharedPods       []*Pod
}

func (c *Cluster) IsNative() bool {
	return !c.Virtual && !c.Shared
}

func (c *Cluster) add(newCluster *Cluster) {
	newCluster.OwnerCluster = c
	c.Nested[newCluster.Name] = newCluster
}

func (c *Cluster) GetCluster(name string) *Cluster {
	if cluster, ok := c.Nested[name]; ok {
		return cluster
	} else {
		for _, cluster := range c.Nested {
			foundCluster := cluster.GetCluster(name)
			if foundCluster != nil {
				return foundCluster
			}
		}
	}
	return nil
}

func (c *Cluster) GetServers() []*Server {
	return lo.Values(c.Servers)
}

func (c *Cluster) GetClusters() []*Cluster {
	return lo.Values(c.Nested)
}

func (c *Cluster) GetSharedClusters() []*Cluster {
	return lo.Reduce(lo.Values(c.Nested), func(agg []*Cluster, item *Cluster, index int) []*Cluster {
		if item.Shared {
			return append(agg, item)
		} else {
			return append(agg, item.GetSharedClusters()...)
		}
	}, make([]*Cluster, 0))
}

func (c *Cluster) getNestedClusterNames() []string {
	return c.getNestedLabels("cluster-name", func(cluster *Cluster) string {
		return cluster.Name
	})
}

func (c *Cluster) getNestedLabels(label string, extract func(*Cluster) string) []string {
	result := make([]string, 0)
	if !c.IsNative() {
		result = append(result, fmt.Sprintf("'%s:%s'", label, extract(c)))
	}

	return lo.Reduce(lo.Values(c.Nested), func(agg []string, item *Cluster, index int) []string {
		return append(agg, item.getNestedLabels(label, extract)...)
	}, result)

}
