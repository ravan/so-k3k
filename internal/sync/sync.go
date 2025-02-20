package sync

import (
	"fmt"
	"github.com/ravan/so-k3k/internal/config"
	"github.com/ravan/so-k3k/internal/k3k"
	"github.com/ravan/stackstate-client/stackstate/receiver"
	"github.com/samber/lo"
	"log/slog"
)

const (
	Source = "k3k"
	Pod    = "pod"
)

func Sync(conf *config.Configuration) (*receiver.Factory, error) {
	factory := receiver.NewFactory(Source, Source, "")
	clusters, err := k3k.New(conf).ConsolidateModel()
	if err != nil {
		return nil, err
	}
	lo.ForEach(clusters, func(nc *k3k.Cluster, index int) {
		processNestedClusters(nc, factory)
	})
	return factory, nil
}

func processNestedClusters(c *k3k.Cluster, f *receiver.Factory) {
	lo.ForEach(c.GetClusters(), func(vc *k3k.Cluster, index int) {
		processVirtualCluster(vc, f)
	})
}

func processVirtualCluster(vc *k3k.Cluster, f *receiver.Factory) {
	lo.ForEach(vc.GetServers(), func(s *k3k.Server, index int) {
		mapServerAndNodeRelation(s, f)
	})
	if vc.Shared {
		lo.ForEach(vc.SharedPods, func(p *k3k.Pod, index int) {
			mapPodRelation(p, f)
		})
		mapVirtualNodeToPodRelation(vc.OwnerClusterName, vc.VirtualNode, f)
	}
	processNestedClusters(vc, f)
}

func mapServerAndNodeRelation(s *k3k.Server, f *receiver.Factory) {
	sComp := f.MustNewComponent(s.MustGetIdentifier(), s.Name, Pod)
	sComp.Data.Layer = "K3K Servers"
	sComp.Data.Domain = s.OwnerClusterName
	commonSeverAndNodeLabels(s, sComp)

	if len(s.NodeIdentifiers) == 0 {
		slog.Warn("Server has no node identifier", "server", s.Name, "identifier", s.MustGetIdentifier())
		return
	}

	nComp := f.MustNewComponent(s.NodeIdentifiers[0], s.NodeName, "node")
	nComp.Data.Domain = s.ClusterName
	if s.NodeAgent {
		nComp.Data.Layer = "K3K Virtual Nodes"
	} else {
		nComp.Data.Layer = "Nodes"
	}
	nComp.Data.Identifiers = s.NodeIdentifiers
	commonSeverAndNodeLabels(s, nComp)

	f.MustNewRelation(nComp.ID, sComp.ID, "is hosted on")
}

func commonSeverAndNodeLabels(s *k3k.Server, comp *receiver.Component) {
	if s.NodeAgent {
		comp.AddLabel("k3k-mode:shared")
		comp.AddProperty("k3kMode", "shared")
	} else {
		comp.AddLabel("k3k-mode:virtual")
		comp.AddProperty("k3kMode", "virtual")
	}
	comp.AddLabelKey("k3k-host-cluster", s.OwnerClusterName)
	comp.AddLabelKey("k3k-host-namespace", s.OwnerClusterNamespace)
	comp.AddProperty("k3kHostClusterIdentifier", fmt.Sprintf("urn:cluster:/kubernetes:%s", s.ClusterName))
	comp.AddProperty("k3kHostNameSpaceIdentifier", fmt.Sprintf("urn:kubernetes:/%s:namespace/%s", s.ClusterName, s.OwnerClusterNamespace))
}

func mapVirtualNodeToPodRelation(ownerClusterName string, vn *k3k.VirtualNode, f *receiver.Factory) {
	if vn.PodIdentifier == "" {
		slog.Warn("Virtual node has no pod identifier", "node", vn.NodeName)
		return
	}
	nComp := f.MustGetComponent(vn.NodeIdentifier)
	pComp := f.MustNewComponent(vn.PodIdentifier, vn.PodName, Pod)
	pComp.Data.Layer = "Pods"
	pComp.Data.Domain = ownerClusterName
	pComp.AddLabel("k3k-mode:shared")
	pComp.AddProperty("k3kPodRole", "host")
	f.MustNewRelation(nComp.ID, pComp.ID, "scheduled_on")
}

func mapPodRelation(p *k3k.Pod, f *receiver.Factory) {
	pComp := f.MustNewComponent(p.Identifier, p.Name, Pod)
	pComp.Data.Layer = "Virtual Pods"
	pComp.Data.Domain = p.ClusterName
	pComp.AddLabel("k3k-mode:shared")
	pComp.AddLabelKey("k3k-host-cluster", p.OwnerClusterName)
	pComp.AddLabelKey("k3k-host-namespace", p.OwnerNamespace)
	pComp.AddLabelKey("k3k-host-podname", p.SharedPodName)
	pComp.AddProperty("k3kHostPodIdentifier", p.SharedPodUID)
	pComp.AddProperty("k3kHostClusterIdentifier", fmt.Sprintf("urn:cluster:/kubernetes:%s", p.ClusterName))
	pComp.AddProperty("k3kHostNameSpaceIdentifier", fmt.Sprintf("urn:kubernetes:/%s:namespace/%s", p.ClusterName, p.OwnerNamespace))
	pComp.AddProperty("k3kPodRole", "share")

	sComp := f.MustNewComponent(p.SharedPodUID, p.SharedPodName, Pod)
	sComp.Data.Layer = "Pods"
	sComp.Data.Domain = p.OwnerClusterName
	sComp.AddLabel("k3k-host-pod")
	sComp.AddProperty("k3kPodRole", "host")
	f.MustNewRelation(pComp.ID, sComp.ID, "scheduled_on")
}
