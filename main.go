package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var statefulSetName string
var headlessService string

func main() {
	waitForCouch()
	exitIfCouchConfigured()

	enableCluster()

	if isLastNode() == true {
		configureCluster()
		finishCluster() }

	log.Info("Configuration completed")
	select{}  // sleep forever
}

func waitForCouch() {
	response, err := http.Get(couchdbServiceURL())
	log.Debug("Waiting for CouchDB")
	for (err != nil) || (response.Status != "200 OK") {
		time.Sleep(time.Second)
		log.Debug(".")
		response, err = http.Get(couchdbServiceURL()) }
}

func exitIfCouchConfigured() {
	req, err := http.NewRequest("GET", couchdbServiceURL() + "/_users", nil)
	panicIfError(err)
	resp, err := httpClient().Do(req)
	panicIfError(err)
	defer resp.Body.Close()

	if ( resp.StatusCode == 200 ){
		log.Info("CouchDB appears to be configured. Sleeping.")
		select{} } }


func expectedReplicaCount() (int) {
	type StatefulSetSpec struct {
		Replicas int `json:"replicas"` }
	type StatefulSetStatus struct {
		Kind string          `json:"kind"`
		Spec StatefulSetSpec `json:"spec"` }

	couchStatefulsetStatusURI := fmt.Sprintf("https://%s:%s/apis/apps/v1beta1/namespaces/%s/statefulsets/couchdb/status",
																					 os.Getenv("KUBERNETES_SERVICE_HOST"),
																					 os.Getenv("KUBERNETES_SERVICE_PORT"),
																					 namespace())

	req, err := http.NewRequest("GET", couchStatefulsetStatusURI, nil)
	panicIfError(err)
	if len(kubernetesAPIToken()) > 0 {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", kubernetesAPIToken())) }

	resp, err := httpClient().Do(req)
	panicIfError(err)
	defer resp.Body.Close()

	if ( resp.StatusCode == 200 ){
		body, err := ioutil.ReadAll(resp.Body)
		panicIfError(err)
		var statusResponse StatefulSetStatus
		json.Unmarshal(body, &statusResponse)
		return statusResponse.Spec.Replicas
	} else {
		log.Debug(couchStatefulsetStatusURI)
		log.Fatal("Could not determine the total replicas for Couch StatefulSet. Response: ", resp) }

	return 0
}

func isLastNode() (bool) {
	lastHostname := fmt.Sprintf("%s-%d", statefulSetName, expectedReplicaCount() - 1)
	return lastHostname == os.Getenv("HOSTNAME")
}

/* Couch helpers */
func couchdbUser() (user string) {
	if user = os.Getenv("COUCHDB_USER"); user == "" {
		user = "admin" }
	return }
func couchdbPassword() (password string) {
	if password = os.Getenv("COUCHDB_PASSWORD"); password == "" {
		password = "password" }
	return }


var clusterSetupURL = "http://localhost:5984/_cluster_setup"
func enableCluster() {
	type EnableClusterStruct struct {
		Action string      `json:"action"`
		BindAddress string `json:"bind_address"`
		Username string    `json:"username"`
		Password string    `json:"password"`
		NodeCount int      `json:"node_count"`}
	jsonStruct := EnableClusterStruct{ Action: "enable_cluster",
																		 BindAddress: "0.0.0.0",
																		 Username: couchdbUser(),
																		 Password: couchdbPassword(),
																		 NodeCount: expectedReplicaCount() }
	buffer := new(bytes.Buffer)
	json.NewEncoder(buffer).Encode(jsonStruct)

	postToCouch(clusterSetupURL, buffer)
	// req, err := http.NewRequest("POST", clusterSetupURL, buffer)
	// req.SetBasicAuth(couchdbUser(), couchdbPassword())
	// req.Header.Add("Content-Type", "application/json")
	// resp, err := httpClient().Do(req)
	// panicIfError(err)
	// defer resp.Body.Close()
	// log.Debug("Enable cluster response: ", resp)
}

func postToCouch(url string, buffer *bytes.Buffer) {
	req, err := http.NewRequest("POST", url, buffer)
	req.SetBasicAuth(couchdbUser(), couchdbPassword())
	req.Header.Add("Content-Type", "application/json")
	resp, err := httpClient().Do(req)
	panicIfError(err)
	defer resp.Body.Close()
	log.Debug("cluster post response: ", resp)
}


func configureCluster() {
	type EnableRemoteNodeStruct struct {
		Action string      `json:"action"`
		BindAddress string `json:"bind_address"`
		Username string    `json:"username"`
		Password string    `json:"password"`
		Port int           `json:"port"`
		NodeCount int      `json:"node_count"`
		RemoteNode string  `json:"remote_node"`
		RemoteCurrentUser string     `json:"remote_current_user"`
		RemoteCurrentPassword string `json:"remote_current_password"` }
	type AddNodeStruct struct {
		Action string   `json:"action"`
		Host string     `json:"host"`
		Port int        `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`}

	petsetDomain := headlessService + "." + namespace() + ".svc.cluster.local"

	for i:=0; i < expectedReplicaCount(); i++ {
		nodeAddress := fmt.Sprintf("%s-%d.%s", statefulSetName, i, petsetDomain)
		log.Info("Adding ", nodeAddress, " to cluster")
		enableStruct := EnableRemoteNodeStruct{
			Action:      "enable_cluster",
			BindAddress: "0.0.0.0",
			Username:    couchdbUser(),
			Password:    couchdbPassword(),
			Port:        couchdbTargetPort(),
			NodeCount:   expectedReplicaCount(),
			RemoteNode:  nodeAddress,
			RemoteCurrentUser:     couchdbUser(),
			RemoteCurrentPassword: couchdbPassword() }

		buffer := new(bytes.Buffer)
		json.NewEncoder(buffer).Encode(enableStruct)
		postToCouch(clusterSetupURL, buffer)
		// req, err := http.NewRequest("POST", clusterSetupURL, buffer)
		// req.SetBasicAuth(couchdbUser(), couchdbPassword())
		// req.Header.Add("Content-Type", "application/json")
		// client := &http.Client{}
		// resp, err := client.Do(req)
		// panicIfError(err)
		// resp.Body.Close()
		// log.Debug("EnableRemoteNode response:", resp)

		addNodeStruct := AddNodeStruct{
			Action:   "add_node",
			Host:     nodeAddress,
			Port:     couchdbTargetPort(),
			Username: couchdbUser(),
			Password: couchdbPassword() }

		buffer = new(bytes.Buffer)
		json.NewEncoder(buffer).Encode(addNodeStruct)
		postToCouch(clusterSetupURL, buffer)
		// req, err = http.NewRequest("POST", clusterSetupURL, buffer)
		// req.SetBasicAuth(couchdbUser(), couchdbPassword())
		// req.Header.Add("Content-Type", "application/json")
		// resp, err = httpClient().Do(req)
		// panicIfError(err)
		// defer resp.Body.Close()
		// log.Debug("AddNode response: ", resp)
	}
}

func finishCluster() {
	type FinishClusterStruct struct {
		Action string `json:"action"` }

	jsonStruct := FinishClusterStruct{ Action: "finish_cluster" }
	buffer := new(bytes.Buffer)
	json.NewEncoder(buffer).Encode(jsonStruct)
	req, err := http.NewRequest("POST", clusterSetupURL, buffer)
	req.SetBasicAuth(couchdbUser(), couchdbPassword())
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpClient().Do(req)
	panicIfError(err)
	defer resp.Body.Close()
	log.Debug("FinishCluster response: ", resp)
}



var couchdbServiceURLVar string
func couchdbServiceURL() string {
	if couchdbServiceURLVar == "" {
		port := os.Getenv(couchdbServiceName() + "_PORT")
		couchdbServiceURLVar = strings.Replace(port, "tcp", "http", 1)
		log.Debug("Using ", couchdbServiceURLVar, " for CouchDB service URL") }
	return couchdbServiceURLVar }

/* The actual port CouchDB is listening on. This could be different from the couch service port (i.e., port vs targetPort) */
// FIXME this assumes service and targetport are the same; should really query the service for the actual target port
var couchdbTargetPortVar int
func couchdbTargetPort() int {
	if couchdbTargetPortVar == 0 {
		couchdbTargetPortVar, _ = strconv.Atoi(os.Getenv(couchdbServiceName() + "_SERVICE_PORT"))
		log.Debug("Using ", couchdbTargetPortVar, " for CouchDB service port") }

	log.Debug("Using ", couchdbTargetPortVar, " for CouchDB service port")

	return couchdbTargetPortVar }


/* Searches the environment for the service corresponding the the "couchdb-port" port.
	 e.g.,  "kind": "Service", "spec": { "ports": [{ "name": "couchdb-port", "port": 5984 }] } */
func couchdbServiceName() string {
	exactRegex := regexp.MustCompile("_SERVICE_PORT_COUCHDB_PORT=[0-9]+")
	fuzzyRegex := regexp.MustCompile("_SERVICE_PORT_.+=5984")
	var fuzzyMatch string
	for _, pair := range os.Environ() {
		if matched := exactRegex.MatchString(pair); matched == true {
			exactMatch := pair[:exactRegex.FindStringIndex(pair)[0]]
			return exactMatch }
		if matched := fuzzyRegex.MatchString(pair); matched == true {
			fuzzyMatch = pair[:fuzzyRegex.FindStringIndex(pair)[0]] } }

	/* note the return above; if we're here, we looped over everything andcouldn't make a match off of "couchdb-port".
		 As a fall-back, we'll assume couch is listening on 5984 with the less exact match */
	if fuzzyMatch != "" {
		return fuzzyMatch
	} else {
		log.Error("Could not determine the kubernetes service name for CouchDB. When using a port other than 5984, you must " +
							"name it 'couchdb-port' for the clustering sidecar to work")
		panic("Could not determine CouchDB Service")
		return "" }
}


/* Kubernetes helpers */

/* Retrieve the kubernetes namespace we're running in. */
func namespace() (namespace string) {
	if namespace = os.Getenv("NAMESPACE"); namespace != "" {
		return
	} else {
		content, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		panicIfError(err)
		namespace = string(content) }
	return
}

func kubernetesCACertificate() (caCert []byte) {
	var err error
	if path := os.Getenv("CA_CERTIFICATE_PATH"); path != "" {
		caCert, err = ioutil.ReadFile(path)
	} else {
		caCert, err = ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt") }
	panicIfError(err)
	return caCert }

func kubernetesAPIToken() (token []byte) {
	if os.Getenv("TOKEN") == "" {
		token, _ = ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	} else {
		token = []byte(os.Getenv("TOKEN")) }
	return
}

/* Random helpers */
func panicIfError(err error){
	if err != nil {
		panic(err.Error()) }
}

var httpClientVar *http.Client
func httpClient() *http.Client {
	if httpClientVar == nil {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(kubernetesCACertificate())

		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool } }

		httpClientVar = &http.Client{
			Transport: transport,
			Timeout: time.Second * 10 } }

	return httpClientVar }

func configureLogger() {
	log.SetOutput(os.Stdout)
	switch level := os.Getenv("LOG_LEVEL"); strings.ToLower(level) {
	case "debug": log.SetLevel(log.DebugLevel)
	case "info":	log.SetLevel(log.InfoLevel)
	case "error": log.SetLevel(log.ErrorLevel)
	case "fatal": log.SetLevel(log.FatalLevel)
	default:      log.SetLevel(log.WarnLevel) }
}


func setOptions() {
	if os.Getenv("SET_NAME") != "" {
		statefulSetName = os.Getenv("SET_NAME")
	} else { 	/* If statefulset name is not passed in, infer from hostname */
		regex := regexp.MustCompile("-[0-9]+$")
		hostname := os.Getenv("HOSTNAME")
		statefulSetName = hostname[:regex.FindStringIndex(hostname)[0]] }

	if os.Getenv("HEADLESS_SERVICE_NAME") != "" {
		headlessService = os.Getenv("HEADLESS_SERVICE_NAME")
	} else {
		headlessService = "couchdb-internal" } }


func init(){
	setOptions()
	configureLogger()
}
