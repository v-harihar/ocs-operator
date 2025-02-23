package collectors

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/red-hat-storage/ocs-operator/metrics/v4/internal/options"
	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	cephv1listers "github.com/rook/rook/pkg/client/listers/ceph.rook.io/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

var (
	mockCephCluster1 = cephv1.CephCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "ceph.rook.io/v1",
			Kind:       "CephCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mockCephCluster-1",
			Namespace: "openshift-storage",
		},
		Spec:   cephv1.ClusterSpec{},
		Status: cephv1.ClusterStatus{},
	}
	mockCephCluster2 = cephv1.CephCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mockCephCluster-2",
			Namespace: "openshift-storage",
		},
		Spec:   cephv1.ClusterSpec{},
		Status: cephv1.ClusterStatus{},
	}
)

func (c *CephClusterCollector) GetInformer() cache.SharedIndexInformer {
	return c.Informer
}

func getMockCephClusterCollector(t *testing.T, mockOpts *options.Options) (mockCephClusterCollector *CephClusterCollector) {
	setKubeConfig(t)
	mockCephClusterCollector = NewCephClusterCollector(mockOpts)
	assert.NotNil(t, mockCephClusterCollector)
	return
}

func TestNewCephClusterCollector(t *testing.T) {
	tests := Tests{
		{
			name: "Test CephClusterCollector",
			args: args{
				opts: mockOpts,
			},
		},
	}
	for _, tt := range tests {
		got := getMockCephClusterCollector(t, tt.args.opts)
		assert.NotNil(t, got.AllowedNamespaces)
		assert.NotNil(t, got.Informer)
	}
}

func TestGetAllCephCluster(t *testing.T) {
	mockOpts.StopCh = make(chan struct{})
	defer close(mockOpts.StopCh)

	cephClusterCollector := getMockCephClusterCollector(t, mockOpts)

	tests := Tests{
		{
			name: "CephCluster doesn't exist",
			args: args{
				lister:     cephv1listers.NewCephClusterLister(cephClusterCollector.Informer.GetIndexer()),
				namespaces: cephClusterCollector.AllowedNamespaces,
			},
			inputObjects: []runtime.Object{},
			// []*cephv1.CephCluster(nil) is not DeepEqual to []*cephv1.CephCluster{}
			// GetAllCephClusters returns []*cephv1.CephCluster(nil) if no CephCluster is present
			wantObjects: []runtime.Object(nil),
		},
		{
			name: "One CephCluster exists",
			args: args{
				lister:     cephv1listers.NewCephClusterLister(cephClusterCollector.Informer.GetIndexer()),
				namespaces: cephClusterCollector.AllowedNamespaces,
			},
			inputObjects: []runtime.Object{
				&mockCephCluster1,
			},
			wantObjects: []runtime.Object{
				&mockCephCluster1,
			},
		},
		{
			name: "Two CephCluster exists",
			args: args{
				lister:     cephv1listers.NewCephClusterLister(cephClusterCollector.Informer.GetIndexer()),
				namespaces: cephClusterCollector.AllowedNamespaces,
			},
			inputObjects: []runtime.Object{
				&mockCephCluster1,
				&mockCephCluster2,
			},
			wantObjects: []runtime.Object{
				&mockCephCluster1,
				&mockCephCluster2,
			},
		},
	}
	for _, tt := range tests {
		setInformer(t, tt.inputObjects, cephClusterCollector)
		gotCephClusters := getAllCephClusters(tt.args.lister.(cephv1listers.CephClusterLister), tt.args.namespaces)
		assert.Len(t, gotCephClusters, len(tt.wantObjects))
		for _, obj := range gotCephClusters {
			assert.Contains(t, tt.wantObjects, obj)
		}
		resetInformer(t, tt.inputObjects, cephClusterCollector)
	}
}

func TestCollectMirrorinDaemonCount(t *testing.T) {
	mockOpts.StopCh = make(chan struct{})
	defer close(mockOpts.StopCh)

	cephMirroringDaemonCountCollector := getMockCephClusterCollector(t, mockOpts)

	objUp1 := mockCephCluster1.DeepCopy()
	objUp1.Name = objUp1.Name + "up1"
	objUp1.Status = cephv1.ClusterStatus{
		CephStatus: &cephv1.CephStatus{
			Versions: &cephv1.CephDaemonsVersions{
				RbdMirror: map[string]int{"ceph version 16.2.5 (0883bdea7337b95e4b611c768c0279868462204a) pacific (stable)": 1},
			},
		},
	}

	// conrnor case
	objUp2 := mockCephCluster1.DeepCopy()
	objUp2.Name = objUp2.Name + "up2"
	objUp2.Status = cephv1.ClusterStatus{
		CephStatus: &cephv1.CephStatus{
			Versions: &cephv1.CephDaemonsVersions{
				RbdMirror: map[string]int{
					"ceph version 16.2.6 (0883bdea7337b95e4b611c768c0279868462204a) pacific (stable)": 1,
					"ceph version 16.2.7 (0883bdea7337b95e4b611c768c0279868462204a) pacific (stable)": 2,
				},
			},
		},
	}

	objDown1 := mockCephCluster1.DeepCopy()
	objDown1.Name = objDown1.Name + "down1"
	objDown1.Status = cephv1.ClusterStatus{
		CephStatus: &cephv1.CephStatus{},
	}

	objDown2 := mockCephCluster1.DeepCopy()
	objDown2.Name = objDown2.Name + "down2"
	objDown2.Status = cephv1.ClusterStatus{}

	tests := Tests{
		{
			name: "Collect Cmirroring daemon count metrics",
			args: args{
				objects: []runtime.Object{
					objUp1,
					objUp2,
					objDown1,
					objDown2,
				},
			},
		},
		{
			name: "Empty CephCluster",
			args: args{
				objects: []runtime.Object{},
			},
		},
	}
	// daemon count
	for _, tt := range tests {
		ch := make(chan prometheus.Metric)
		metric := dto.Metric{}
		go func() {
			var cephClusters []*cephv1.CephCluster
			for _, obj := range tt.args.objects {
				cephClusters = append(cephClusters, obj.(*cephv1.CephCluster))
			}
			cephMirroringDaemonCountCollector.collectMirrorinDaemonCount(cephClusters, ch)
			close(ch)
		}()

		for m := range ch {
			assert.Contains(t, m.Desc().String(), "count")
			metric.Reset()
			err := m.Write(&metric)
			assert.Nil(t, err)
			labels := metric.GetLabel()
			for _, label := range labels {
				if *label.Name == "ceph_cluster" {
					if *label.Value == objUp1.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(1))
					} else if *label.Value == objUp2.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(3))
					} else if *label.Value == objDown1.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(0))
					} else if *label.Value == objDown2.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(0))
					}
				} else if *label.Name == "namespace" {
					assert.Contains(t, cephMirroringDaemonCountCollector.AllowedNamespaces, *label.Value)
				}
			}
		}
	}

}

func TestCollectNodeLabels(t *testing.T) {
	mockOpts.StopCh = make(chan struct{})
	defer close(mockOpts.StopCh)

	cephClusterCollector := getMockCephClusterCollector(t, mockOpts)

	cephNodeList := &corev1.NodeList{
		TypeMeta: metav1.TypeMeta{Kind: "NodeList"},
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "Node1",
					Labels: map[string]string{
						"cluster.ocs.openshift.io/openshift-storage": "",
						"failure-domain.beta.kubernetes.io/zone":     "zonea",
						"hostnameLabel":                              "node1",
					},
				},
				Spec: corev1.NodeSpec{
					ProviderID: "aws://x",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "Node2",
					Labels: map[string]string{
						"cluster.ocs.openshift.io/openshift-storage": "",
						"failure-domain.beta.kubernetes.io/zone":     "",
						"hostnameLabel":                              "node2",
					},
				},
				Spec: corev1.NodeSpec{
					ProviderID: "aws",
				},
			},
		},
	}

	ch := make(chan prometheus.Metric)
	metric := dto.Metric{}
	go func() {
		cephClusterCollector.collectNodeLabels(cephNodeList, ch)
		close(ch)
	}()

	for m := range ch {
		assert.Contains(t, m.Desc().String(), "ocs_node_labels")
		metric.Reset()
		err := m.Write(&metric)
		assert.Nil(t, err)
		labels := metric.GetLabel()
		assert.Equal(t, len(labels), 3)
		for _, label := range labels {
			if *label.Name == "label_provider_id" {
				assert.True(t, *label.Value == "aws" || *label.Value == "")
			}
			if *label.Name == "label_failure_domain_beta_kubernetes_io_zone" {
				assert.True(t, *label.Value == "zonea" || *label.Value == "")
			}
			if *label.Name == "hostnameLabel" {
				assert.True(t, *label.Value == "node1" || *label.Value == "node2")
			}
		}
	}
}
