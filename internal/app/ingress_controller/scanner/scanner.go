// Scanner pulls information from the kubernetes cluster using the API running locally on the machine.
package scanner

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strconv"
	"time"
)

// MyServices contains data for all services in the network
type MyServices struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		SelfLink        string `json:"selfLink"`
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Items []Service `json:"items"`
}

// Service struct contains data pertaining to a service
type Service struct {
	Metadata struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		SelfLink          string    `json:"selfLink"`
		UID               string    `json:"uid"`
		ResourceVersion   string    `json:"resourceVersion"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
		Labels            struct {
			Component string `json:"component"`
			Provider  string `json:"provider"`
		} `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Ports []struct {
			Protocol   string `json:"protocol"`
			Port       int    `json:"port"`
			TargetPort int    `json:"targetPort"`
		} `json:"ports"`
		ClusterIP       string `json:"clusterIP"`
		Type            string `json:"type"`
		SessionAffinity string `json:"sessionAffinity"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
		} `json:"loadBalancer"`
	} `json:"status"`
}

// Portal retrieves data of portal information
type Portal struct {
	APIVersion string `json:"apiVersion"`
	Items      []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Annotations struct {
				KubectlKubernetesIoLastAppliedConfiguration string `json:"kubectl.kubernetes.io/last-applied-configuration"`
			} `json:"annotations"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
			Generation        int       `json:"generation"`
			Name              string    `json:"name"`
			Namespace         string    `json:"namespace"`
			ResourceVersion   string    `json:"resourceVersion"`
			SelfLink          string    `json:"selfLink"`
			UID               string    `json:"uid"`
		} `json:"metadata"`
		Spec struct {
			Portal   string `json:"portal"`
			Targetip string `json:"targetip"`
		} `json:"spec"`
	} `json:"items"`
	Kind     string `json:"kind"`
	Metadata struct {
		Continue        string `json:"continue"`
		ResourceVersion string `json:"resourceVersion"`
		SelfLink        string `json:"selfLink"`
	} `json:"metadata"`
}

// IngressData stores
type IngressData struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		SelfLink        string `json:"selfLink"`
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Items []IngressItem `json:"items"`
}

// IngressItem stores
type IngressItem struct {
	Metadata struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		SelfLink          string    `json:"selfLink"`
		UID               string    `json:"uid"`
		ResourceVersion   string    `json:"resourceVersion"`
		Generation        int       `json:"generation"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec struct {
		TLS []struct {
			Hosts      []string `json:"hosts"`
			SecretName string   `json:"secretName"`
		} `json:"tls"`
		Rules []IngressRules `json:"rules"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
		} `json:"loadBalancer"`
	} `json:"status"`
}

// IngressRules stores
type IngressRules struct {
	Host string `json:"host"`
	Prot struct {
		Paths []struct {
			Path    string `json:"path"`
			Backend struct {
				ServiceName string `json:"serviceName"`
				ServicePort int    `json:"servicePort"`
			} `json:"backend"`
		} `json:"paths"`
	} `json:"http"`
}

// Route stores
type Route struct {
	ServiceName string `json:"ServiceName"`
	ServicePort string `json:"ServicePort"`
	ServiceIP   string `json:"ServiceIP"`
}

// Rules stores
type Rules struct {
	Protocol string `json:"Protocol"`
	Path     string `json:"Path"`
	Route    Route  `json:"Route"`
}

// AltCluster stores
type AltCluster struct {
	ClusterName string
	ClusterIP   string
	ClusterPort string
}

// Ruleset stores
var Ruleset []Rules

// ReqServices contians
var ReqServices MyServices

// TargetIP will store alternative IP address to dial if first one is not found
var TargetIP []AltCluster

// Scan gets the rules and clusters from the Kubernetes API and returns them as structs
// Intended to be called by the proxy
func Scan() (ruleset []Rules, targetIP []AltCluster) {
	// run the kubectl proxy without TLS credentials
	TargetIP = []AltCluster{}
	Ruleset = []Rules{}
	exec.Command("kubectl", "proxy", "--insecure-skip-tls-verify").Start()
	fmt.Println("Kube Proxy Running")
	time.Sleep(5 * time.Second)
	fmt.Println("Kube Proxy up")
	GetTargetIP()
	GetIngress()
	return Ruleset, TargetIP
}

// GetServices gets all of the services in our cluster from the API
func GetServices(serviceName string) (clusterIP string, clusterPort string) {

	// request information of services from k8s API
	serviceURL := "http://localhost:8001/api/v1/services"
	body, err := GetResponse(serviceURL)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// unmarshall body of the request and populate structure currServices with information of current services from K8S API
	err = json.Unmarshal(body, &ReqServices)
	if err != nil {
		fmt.Println(err)
	}
	clusterIP, clusterPort = FindService(serviceName)

	return
}

//FindService searches list of services by 'name' to match
func FindService(serviceName string) (clusterIP string, clusterPort string) {

	serviceLst := ReqServices.Items
	for i := 0; i < len(serviceLst); i++ {
		currService := serviceLst[i]
		fmt.Println("Looking for: ", serviceName)
		fmt.Println("currently on: ", currService.Metadata.Name)
		if currService.Metadata.Name == serviceName {
			clusterIP = currService.Spec.ClusterIP
			clusterPort = strconv.Itoa(currService.Spec.Ports[0].Port)
			fmt.Println("clusterIP = ", clusterIP)
			fmt.Println("clusterPort = ", clusterPort)
			return
		}
	}
	return
}

// GetIngress contains
func GetIngress() {

	// items.spec, items.rules, items.http, items.path, items.sepc.ruleshttp.paths.backend.serviceport == serviceport, items.sepc.ruleshttp.paths.backend.servicename = servicename serviceip == cluster ip
	var TargetData = IngressData{}
	var MyIngress = []IngressRules{}
	var MyRoute = Route{}
	var MyRules = Rules{}
	serviceURL := "http://localhost:8001/apis/extensions/v1beta1/ingresses"

	body, err := GetResponse(serviceURL)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = json.Unmarshal(body, &TargetData)
	if err != nil {
		fmt.Println(err)
		fmt.Println("This is where We think the error is during unmarshal of ingress")
		return
	}

	json.Unmarshal([]byte(body), &TargetData)
	for i := 0; i < len(TargetData.Items); i++ {
		myitem := TargetData.Items[i]
		if TargetData.Items[i].Metadata.Name == "ingressname" {
			MyIngress = myitem.Spec.Rules
			break
		}

	}

	for i := 0; i < len(MyIngress); i++ {

		for k := range MyIngress[i].Prot.Paths {
			serviceIP, servicePort := GetServices(MyIngress[i].Prot.Paths[k].Backend.ServiceName)

			MyRoute.ServiceName = MyIngress[i].Prot.Paths[k].Backend.ServiceName
			MyRoute.ServicePort = servicePort
			MyRoute.ServiceIP = serviceIP
			MyRules.Path = MyIngress[i].Prot.Paths[k].Path
			MyRules.Route = MyRoute
			MyRules.Protocol = "http"
			Ruleset = append(Ruleset, MyRules)
		}
	}

}

// GetTargetIP will retrieve targetIP from the portal to provide an alternative IP address for proxy
func GetTargetIP() {
	// request information of services from k8s API
	var PortalData Portal
	var MyCluster AltCluster
	serviceURL := "http://localhost:8001/apis/revature.com/v1/portals/"
	body, err := GetResponse(serviceURL)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// unmarshall body of the request and populate structure currServices with information of current services from K8S API
	err = json.Unmarshal(body, &PortalData)
	if err != nil {
		fmt.Println(err)
	}
	if len(PortalData.Items) != 0 {
		MyCluster.ClusterName = PortalData.Items[0].Metadata.Name
		MyCluster.ClusterPort = "80"
		MyCluster.ClusterIP = PortalData.Items[0].Spec.Targetip
		TargetIP = append(TargetIP, MyCluster)
	}
}

// GetResponse will request response from Kubernates API
func GetResponse(requestURL string) (respBody []byte, err error) {

	// create a new instance of client & create new request to retrieve info from k8s API
	client := http.Client{}
	apiReq, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// client do request: send HTTP request & recieve HTTP response
	response, err := client.Do(apiReq)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// read body of the reponse recieved from k8s API and defer closing body until end
	respBody, err = ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return
}
