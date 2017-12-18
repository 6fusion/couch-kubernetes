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

```bash
~> kubectl apply -f couchdb-statefulset.yml
service "couchdb-internal" created
service "couchdb" created
statefulset "couchdb" created
```

#### Verify Deployment

Since the pods in a StatefulSet deploy sequentially, it will take a moment for the cluster to be ready.
You can watch the set deploy:
```bash
~>watch kubectl get pods

Every 2.0s: kubectl get pods

NAME        READY     STATUS    RESTARTS   AGE
couchdb-0   2/2       Running   0          10m
couchdb-1   2/2       Running   2          10m
```

There may an an ephemeral restart or two, as CouchDB comes up; this is expected.
Once both pods are running, you can verify the cluster membership:
```bash
~> curl -s -u admin:password "$(kubectl get service couchdb -o jsonpath='{.spec.clusterIP}'):5984/_membership" | jq .
{
  "all_nodes": [
    "couchdb@couchdb-0.couchdb-internal.default.svc.cluster.local",
    "couchdb@couchdb-1.couchdb-internal.default.svc.cluster.local"
  ],
  "cluster_nodes": [
    "couchdb@couchdb-0.couchdb-internal.default.svc.cluster.local",
    "couchdb@couchdb-1.couchdb-internal.default.svc.cluster.local"
  ]
}

```

You should need the nodes in both the **all_nodes** and **cluster_nodes** sections.

If you want to be really thorough, you can visit the individual CouchDB instances on their pod network address.
To get the address of each node:
```bash
kubectl get endpoints couchdb -o json | jq -r .subsets[].addresses[].ip
172.17.0.3
172.17.0.4
```

You can test the cluster in a number of ways. As an example:
```bash
~>curl -H "Content-type: application/json" 172.17.0.3:5984/test
{"error":"not_found","reason":"Database does not exist."}

# Note the change in Pod address
~>curl -u admin:password -XPUT -H"Content-type: application/json" 172.17.0.4:5984/test
{"ok":true}

# At this point, curling the first node again should return stats on the test database
~>curl -s -H "Content-type: application/json" 172.17.0.4:5984/test | jq .
{
  "db_name": "test",
  "update_seq": "0-g1AAAAKreJzLYWBg4MhgTmGwSc4vTc5ISXKA0rqGejBWZl5JalFeYo5eSmpaYmlOiV5xWbJeck5pMVBYLyc_OTEnB2gKUyJDkvz___-zEhmwmmdAqnlJCkAyyR6PkSQ7MckBZGQ8VV2ZADKynpquzGMBkgwNQApo6nyquRRi7AKIsfup7NoDEGPvU9m1DyDGgsI2CwDPrfAa",
  "sizes": {
    "file": 34296,
    "external": 0,
    "active": 0
  },
  "purge_seq": 0,
  "other": {
    "data_size": 0
  },
  "doc_del_count": 0,
  "doc_count": 0,
  "disk_size": 34296,
  "disk_format_version": 6,
  "data_size": 0,
  "compact_running": false,
  "instance_start_time": "0"
}

```

You should also see shards for the `test` database distrubuted across the nodes
```bash
for pod in $(echo $PODS); do echo "Pod: $pod"; kubectl exec $pod -c couchdb -- /usr/bin/find /opt/couchdb/data/; done | grep -E '(Pod|test)'
Pod: couchdb-0
/opt/couchdb/data/shards/a0000000-bfffffff/test.1505578429.couch
/opt/couchdb/data/shards/20000000-3fffffff/test.1505578429.couch
/opt/couchdb/data/shards/60000000-7fffffff/test.1505578429.couch
/opt/couchdb/data/shards/e0000000-ffffffff/test.1505578429.couch
Pod: couchdb-1
/opt/couchdb/data/shards/c0000000-dfffffff/test.1505578429.couch
/opt/couchdb/data/shards/80000000-9fffffff/test.1505578429.couch
/opt/couchdb/data/shards/40000000-5fffffff/test.1505578429.couch
/opt/couchdb/data/shards/00000000-1fffffff/test.1505578429.couch
```

#### Limitations
The sidecar is not able to add more nodes once the cluster has been initialized. Currently, you must ensure that the desired number of Pod replicas is set when creating the initial Couch cluster.

## Building the sidecar
**couch-sidecar** is a small go-language application that sets up the CouchDB cluster. While it is already built and available in the
[6fusion/couchdb-sidecar](https://hub.docker.com/r/6fusion/couchdb-sidecar) docker image, you can build it from source if you prefer.
It's a small, fairly standard go-language app that relies on [glide](https://github.com/Masterminds/glide) for dependency management.

To create an executable compatible with the `Dockerfile.sidecar` in this repository, you will need to compile a static binary for Linux.
Assuming you have go installed and configured, you can build the binary by running (from the root of the cloned repository):
```
glide install
GOOS=linux CGO_ENABLED=0 go build -a -ldflags="-s -w" -o couch-sidecar
```
