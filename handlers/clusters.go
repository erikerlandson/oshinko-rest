package handlers

import (
	"fmt"
	"os"
	"strconv"

	middleware "github.com/go-openapi/runtime/middleware"

	_ "github.com/openshift/origin/pkg/api/install"
	serverapi "github.com/openshift/origin/pkg/cmd/server/api"
	ocon "github.com/redhatanalytics/oshinko-rest/helpers/containers"
	odc "github.com/redhatanalytics/oshinko-rest/helpers/deploymentconfigs"
	opt "github.com/redhatanalytics/oshinko-rest/helpers/podtemplates"
	osv "github.com/redhatanalytics/oshinko-rest/helpers/services"
	"github.com/redhatanalytics/oshinko-rest/restapi/operations/clusters"

	"github.com/redhatanalytics/oshinko-rest/models"
	"strconv"
)

func sparkMasterURL(name string, port *osv.OServicePort) string {
	return "spark://" + name + ":" + strconv.Itoa(port.ServicePort.Port)
}

func sparkWorker(namespace string,
	image string,
	replicas int, masterurl string) *odc.ODeploymentConfig {

	// Create the basic deployment config
	dc := odc.DeploymentConfig(
		"spark-worker",
		namespace).TriggerOnConfigChange().RollingStrategy().Replicas(replicas)

	// We will use a "name" label with the name of the deployment config
	// as a selector for the pods controlled by this deployment.
	// Set the selector on the deployment config ...
	dc = dc.PodSelector("name", dc.Name)

	// ... and create a pod template spec with the matching label
	pt := opt.PodTemplateSpec().SetLabels(dc.GetPodSelectors())

	// Create a container with the correct start command
	cont := ocon.Container(
		dc.Name,
		image).Command("/start-worker", masterurl)

	// Finally, assign the container to the pod template spec and
	// assign the pod template spec to the deployment config
	return dc.PodTemplateSpec(pt.Containers(cont))
}

func sparkMaster(namespace string, image string) *odc.ODeploymentConfig {

	// Create the basic deployment config
	// dc.Name will be spark-master-<suffix>
	dc := odc.DeploymentConfig(
		"spark-master",
		namespace).TriggerOnConfigChange().RollingStrategy()

	// We will use a "name" label with the name of the deployment config
	// as a selector for the pods controlled by this deployment.
	// Set the selector on the deployment config ...
	dc = dc.PodSelector("name", dc.Name)

	// ... and create a pod template spec with the matching label
	pt := opt.PodTemplateSpec().SetLabels(dc.GetPodSelectors())

	// Create a container with the correct ports and start command
	masterp := ocon.ContainerPort("spark-master", 7077)
	webp := ocon.ContainerPort("spark-webui", 8080)
	cont := ocon.Container(
		dc.Name,
		image).Command("/start-master",
		dc.Name).Ports(masterp, webp)

	// Finally, assign the container to the pod template spec and
	// assign the pod template spec to the deployment config
	return dc.PodTemplateSpec(pt.Containers(cont))
}

func Service(name string, port int, mylabels, podlabels map[string]string) (*osv.OService, *osv.OServicePort) {
	p := osv.ServicePort(port).TargetPort(port)
	return osv.Service(name).SetLabels(mylabels).PodSelectors(podlabels).Ports(p), p
}

// CreateClusterResponse create a cluster and return the representation
func CreateClusterResponse(params clusters.CreateClusterParams) middleware.Responder {
	// create a cluster here
	// INFO(elmiko) my thinking on creating clusters is that we should use a
	// label on the items we create with kubernetes so that we can get them
	// all with a request.
	// in addition to labels for general identification, we should then use
	// annotations on objects to help further refine what we are dealing with.

	namespace := os.Getenv("OSHIKO_CLUSTER_NAMESPACE")
	configfile := os.Getenv("OSHINKO_KUBE_CONFIG")
	image := os.Getenv("OSHINKO_CLUSTER_IMAGE")
	if namespace == "" || configfile == "" || image == "" {
		payload := makeSingleErrorResponse(501, "Missing Env",
			"OSHIKO_CLUSTER_NAMESPACE, OSHINKO_KUBE_CONFIG, and OSHINKO_CLUSTER_IMAGE env vars must be set")
		return clusters.NewCreateClusterDefault(501).WithPayload(payload)
	}

	// kube rest client
	client, _, err := serverapi.GetKubeClient(configfile)
	if err != nil {
		// handle error
	}

	// openshift rest client
	osclient, _, err := serverapi.GetOpenShiftClient(configfile)
	if err != nil {
		//handle error
	}

	// deployment config client
	dcc := osclient.DeploymentConfigs(namespace)

	// Make master deployment config
	// Ignoring master-count for now, leave it defaulted at 1
	masterdc := sparkMaster(namespace, image)

	// Make master services
	mastersv, masterp := Service(masterdc.Name,
		masterdc.FindPort("spark-master"),
		masterdc.GetPodSelectors(), masterdc.GetPodSelectors())

	websv, _ := Service(masterdc.Name+"webui",
		masterdc.FindPort("spark-webui"),
		masterdc.GetPodSelectors(),
		masterdc.GetPodSelectors())

	// Make worker deployment config
	masterurl := sparkMasterURL(mastersv.Name, masterp)
	workerdc := sparkWorker(
		namespace,
		image,
		int(*params.Cluster.WorkerCount), masterurl)

	// Launch all of the objects
	_, err = dcc.Create(&masterdc.DeploymentConfig)
	if err != nil {
		fmt.Println(err)
	}
	dcc.Create(&workerdc.DeploymentConfig)
	client.Services(namespace).Create(&mastersv.Service)
	client.Services(namespace).Create(&websv.Service)

	cluster := &models.SingleCluster{&models.ClusterModel{}}
	cluster.Cluster.Name = params.Cluster.Name
	cluster.Cluster.WorkerCount = params.Cluster.WorkerCount
	cluster.Cluster.MasterCount = params.Cluster.MasterCount
	return clusters.NewCreateClusterCreated().WithLocation(masterurl).WithPayload(cluster)
}

// DeleteClusterResponse delete a cluster
func DeleteClusterResponse(params clusters.DeleteSingleClusterParams) middleware.Responder {
	payload := makeSingleErrorResponse(501, "Not Implemented",
		"operation clusters.DeleteSingleCluster has not yet been implemented")
	return clusters.NewCreateClusterDefault(501).WithPayload(payload)
}

// FindClustersResponse find a cluster and return its representation
func FindClustersResponse() middleware.Responder {
	payload := makeSingleErrorResponse(501, "Not Implemented",
		"operation clusters.FindClusters has not yet been implemented")
	return clusters.NewCreateClusterDefault(501).WithPayload(payload)
}

// FindSingleClusterResponse find a cluster and return its representation
func FindSingleClusterResponse(clusters.FindSingleClusterParams) middleware.Responder {
	payload := makeSingleErrorResponse(501, "Not Implemented",
		"operation clusters.FindSingleCluster has not yet been implemented")
	return clusters.NewCreateClusterDefault(501).WithPayload(payload)
}

// UpdateSingleClusterResponse update a cluster and return the new representation
func UpdateSingleClusterResponse(params clusters.UpdateSingleClusterParams) middleware.Responder {
	payload := makeSingleErrorResponse(501, "Not Implemented",
		"operation clusters.UpdateSingleCluster has not yet been implemented")
	return clusters.NewCreateClusterDefault(501).WithPayload(payload)
}
