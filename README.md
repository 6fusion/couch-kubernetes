# couch-kubernetes
Clustered CouchDB for Kubernetes

Uses a Kubernetes StatefulSet and a small "sidecar" container to configure a CouchDB cluster.

## Usage
Download the couchdb-statefulset.yml file

```curl https://raw.githubusercontent.com/6fusion/couch-kubernetes/master/couchdb-statefulset.yml```

Update the `replicas` field to reflect the desired number of CouchDB nodes
Update the `storage` field under the `volumeClaimTemplates` to an appropriate value.

Deploy the StatefulSet

```kubectl apply -f couchdb-statefulset.yml```
