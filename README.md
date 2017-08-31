# couch-kubernetes
Clustered CouchDB for Kubernetes

Uses a Kubernetes StatefulSet and a small "sidecar" container to set up a CouchDB cluster.

## Usage
#### Download the couchdb-statefulset.yml file

```curl -O https://raw.githubusercontent.com/6fusion/couch-kubernetes/master/couchdb-statefulset.yml```

#### Edit the downloaded yaml

Update the `replicas` field to reflect the desired number of CouchDB nodes

Update the `storage` field under the `volumeClaimTemplates` to an appropriate value.

Update the environment variables `COUCHDB_USER` and `COUCHDB_PASSWORD` if security is a concern.

If you decide to rename the headless service, you will need to add an environment variable `HEADLESS_SERVICE_NAME` with the new name to both containers' environments.

#### Deploy the StatefulSet

```kubectl apply -f couchdb-statefulset.yml```
