# rabbit-stalker

<img src="https://github.com/prosimcorp/rabbit-stalker/raw/main/docs/img/logo.png" width="100">

Kubernetes Operator to get status related to specific queues on RabbitMQ. Depending on 
some condition, it's able to restart or delete a Kubernetes workload

![Kubernetes](https://img.shields.io/badge/Kubernetes-%3E%3D%201.18-brightgreen)
![GitHub Release](https://img.shields.io/github/v/release/prosimcorp/rabbit-stalker)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/prosimcorp/rabbit-stalker)
[![Go Report Card](https://goreportcard.com/badge/github.com/prosimcorp/rabbit-stalker)](https://goreportcard.com/report/github.com/prosimcorp/rabbit-stalker)
![image pulls](https://img.shields.io/badge/+2k-brightgreen?label=image%20pulls)
![GitHub License](https://img.shields.io/github/license/prosimcorp/rabbit-stalker)

![GitHub User's stars](https://img.shields.io/github/stars/prosimcorp?label=Prosimcorp%20Stars)
![GitHub followers](https://img.shields.io/github/followers/prosimcorp?label=Prosimcorp%20Followers)

----

> **ATTENTION:** From v1.1.0+ bundled Kubernetes deployment manifests are built and uploaded to the releases.
> We do this to keep them atomic between versions. Due to this, `deploy` directory will be removed from repository.
> Please, read [related section](#deployment)

> Hello there! a little advise: we crafted this wonderful project at [DocPlanner Tech](https://www.docplanner.tech/).
> We moved it to ProsimCorp to assure the continuity of the project. Are you willing to contribute?

## Description
This project was motivated by a failure. Most famous PHP/Python libraries can establish connection with AMQP servers
like RabbitMQ. At some point, the connection is randomly broken, but the libraries are not throwing an exception to handle the
failure reconnecting, so in the end, a queue does not have consumers and the application does not know it 
(yes! like zombie workers)

This problem can be solved in many ways, but one of the solutions (from the infra team perspective) can be just 
getting the credentials, doing some requests to RabbitMQ admin API, and depending on the number of consumers, just
restart the related workload. 

This is exactly what this operator does, but with vitamins:
* Includes [GJSON](https://github.com/tidwall/gjson) on conditions to look for a particular field in the huge JSON given by RabbitMQ
* It supports giving credentials (or not) to access RabbitMQ
* Support restarting several workload types: `Deployment` `DaemonSet` `StatefulSet`

Any discussion can be done on issues. Most interesting questions will be included on our [FAQ section](./README.md#faq-frequently-asked-questions)

> Hey, little hint here! you can debug GJSON patterns using the official [debugging website](https://gjson.dev/)

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Knowing the spec
In this section we will show you some examples that maybe helps you. Anyway, the complete working example 
[is located here](./config/samples)

```yaml
apiVersion: rabbit-stalker.prosimcorp.com/v1alpha1
kind: WorkloadAction
metadata:
  name: workloadaction-sample
spec:
  synchronization:
    time: 10s

  # Configuration parameters to be able to connect with RabbitMQ
  rabbitConnection:
    url: "https://your-server.rmq.cloudamqp.com"
    queue: "your-queue-here"
    useRegex: true

    # (Optional) Vhost can be set (or not) when searching queues using regex patterns,
    # (Mandatory) Vhost is required for searches based on exact queue names.
    vhost: "shared"

    # (Optional) Credentials to authenticate against endpoint.
    # If set, both are required
    credentials:
      username:
        secretRef:
          name: testing-secret
          key: RABBITMQ_USERNAME

          # (Optional) Getting credentials from other namespace is possible too
          # When namespace is not defined, the same where this WorkloadAction CR is running will be used
          namespace: default
      password:
        secretRef:
          name: testing-secret
          key: RABBITMQ_PASSWORD

  # (Optional) Additional sources to get information from.
  # This sources can be used on condition.value
  additionalSources:
    - apiVersion: apps/v1
      kind: Deployment
      name: testing-workload
      namespace: default

  # This is the condition that will trigger the execution of the action.
  condition:
    # The 'key' field admits dot notation, and it's covered by gjson
    # Ref: https://github.com/tidwall/gjson
    key: "test"

    # Additional sources from 'additionalSources' field can be used here to craft complex values using the pattern:
    # [index]{{ gjson }}
    value: "something"

  # Action to do with the workload when the condition is met
  action: "restart"

  # The workload affected by the action
  workloadRef:
    apiVersion: apps/v1
    kind: Deployment
    name: testing-workload
    namespace: default
```

#### Queue names/regex

Let's start talking about how to look for queues inside your RabbitMQ. The first approach is just to define a static
name for the `queue` and `vhost`. This will perform the action over the workload when the condition is met, as simple
as follows:

```yaml
apiVersion: rabbit-stalker.prosimcorp.com/v1alpha1
kind: WorkloadAction
metadata:
  name: workloadaction-sample
spec:
  # ...
  rabbitConnection:
    url: "https://your-server.rmq.cloudamqp.com"
    vhost: "shared"
    queue: "your-queue-here"
    useRegex: false
```

But what happens if you have some monolithic application that handle several queues at the same time? if several queues
are managed by the same application instance (or pod inside Kubernetes), it's possible that your queues' names are defined
following a pattern that can be represented by a REGEX expression. In that case, you can use the following feature:

```yaml
apiVersion: rabbit-stalker.prosimcorp.com/v1alpha1
kind: WorkloadAction
metadata:
  name: workloadaction-sample
spec:
  # ...
  rabbitConnection:
    url: "https://your-server.rmq.cloudamqp.com"
    vhost: "shared"
    queue: |-
      ^your_monolith_prefix.(cz|es|it|pl|tr|pt|de)_incoming_events$
    useRegex: true
```

> ATTENTION! If the condition is met for some of them, the action will be immediately executed over the workload

#### Conditions

What can you do with the condition? As we said, it admits GJSON for dot notation, so basically, you can look for any
field inside the RabbitMQ response. As an example, take the following sample JSON

<details>
  <summary>Click me</summary>

  ```json
    {
      "consumer_details": [
        {
          "arguments": {},
          "channel_details": {
            "connection_name": "xxx.xxx.xxx.xxx:51102 -> xx.xx.xx.xxx:5671",
            "name": "xxx.xxx.xxx.xxx:51102 -> xx.xx.xx.xxx:5671 (1)",
            "node": "rabbit@fancy-monk-sample-01",
            "number": 1,
            "peer_host": "xxx.xxx.xxx.xxx",
            "peer_port": 51102,
            "user": "sample-app"
          },
          "ack_required": true,
          "active": true,
          "activity_status": "up",
          "consumer_tag": "xx",
          "exclusive": false,
          "prefetch_count": 30,
          "queue": {
            "name": "sample-app-queue",
            "vhost": "shared"
          }
        },
        {
          "arguments": {},
          "channel_details": {
            "connection_name": "xxx.xxx.xxx.xxx:24752 -> xx.xx.xx.xxx:5671",
            "name": "xxx.xxx.xxx.xxx:51102 -> xx.xx.xx.xxx:5671 (1)",
            "node": "rabbit@fancy-monk-sample-03",
            "number": 1,
            "peer_host": "xxx.xxx.xxx.xxx",
            "peer_port": 24752,
            "user": "sample-app"
          },
          "ack_required": true,
          "active": true,
          "activity_status": "up",
          "consumer_tag": "xx",
          "exclusive": false,
          "prefetch_count": 30,
          "queue": {
            "name": "sample-app-queue",
            "vhost": "shared"
          }
        }
      ],
      "arguments": {
        "x-dead-letter-exchange": "sample-app-toxic"
      },
      "auto_delete": false,
      "backing_queue_status": {
        "avg_ack_egress_rate": 0.07669378573657928,
        "avg_ack_ingress_rate": 0.14652897050941927,
        "avg_egress_rate": 0.14652897050941927,
        "avg_ingress_rate": 0.14652897050941927,
        "delta": [
          "delta",
          "undefined",
          0,
          0,
          "undefined"
        ],
        "len": 0,
        "mirror_seen": 0,
        "mirror_senders": 15,
        "mode": "default",
        "next_seq_id": 14376198,
        "q1": 0,
        "q2": 0,
        "q3": 0,
        "q4": 0,
        "target_ram_count": "infinity"
      },
      "consumer_capacity": 1,
      "consumer_utilisation": 1,
      "consumers": 2,
      "deliveries": [],
      "durable": true,
      "effective_policy_definition": {
        "ha-mode": "exactly",
        "ha-params": 2,
        "ha-sync-mode": "automatic"
      },
      "exclusive": false,
      "exclusive_consumer_tag": null,
      "garbage_collection": {
        "fullsweep_after": 65535,
        "max_heap_size": 0,
        "min_bin_vheap_size": 46422,
        "min_heap_size": 233,
        "minor_gcs": 2
      },
      "head_message_timestamp": null,
      "idle_since": "1995-01-01 23:28:09",
      "incoming": [],
      "memory": 22824,
      "message_bytes": 0,
      "message_bytes_paged_out": 0,
      "message_bytes_persistent": 0,
      "message_bytes_ram": 0,
      "message_bytes_ready": 0,
      "message_bytes_unacknowledged": 0,
      "message_stats": {
        "ack": 14364988,
        "ack_details": {
          "rate": 0
        },
        "deliver": 14373410,
        "deliver_details": {
          "rate": 0
        },
        "deliver_get": 14373410,
        "deliver_get_details": {
          "rate": 0
        },
        "deliver_no_ack": 0,
        "deliver_no_ack_details": {
          "rate": 0
        },
        "get": 0,
        "get_details": {
          "rate": 0
        },
        "get_empty": 0,
        "get_empty_details": {
          "rate": 0
        },
        "get_no_ack": 0,
        "get_no_ack_details": {
          "rate": 0
        },
        "publish": 12085075,
        "publish_details": {
          "rate": 0
        },
        "redeliver": 4659,
        "redeliver_details": {
          "rate": 0
        }
      },
      "messages": 0,
      "messages_details": {
        "rate": 0
      },
      "messages_paged_out": 0,
      "messages_persistent": 0,
      "messages_ram": 0,
      "messages_ready": 0,
      "messages_ready_details": {
        "rate": 0
      },
      "messages_ready_ram": 0,
      "messages_unacknowledged": 0,
      "messages_unacknowledged_details": {
        "rate": 0
      },
      "messages_unacknowledged_ram": 0,
      "name": "sample-app",
      "node": "rabbit@fancy-monk-sample-01",
      "operator_policy": null,
      "policy": "HA",
      "recoverable_slaves": [
        "rabbit@fancy-monk-sample-03"
      ],
      "reductions": 20884973878,
      "reductions_details": {
        "rate": 0
      },
      "single_active_consumer_tag": null,
      "slave_nodes": [
        "rabbit@fancy-monk-sample-03"
      ],
      "state": "running",
      "synchronised_slave_nodes": [
        "rabbit@fancy-monk-sample-03"
      ],
      "type": "classic",
      "vhost": "shared"
    }
  ```
</details>

Simple conditions are fine, so the next example will cover the case where you have 0 (zero) consumers
and need to restart the application because of zombie processes previously mentioned:

```yaml
apiVersion: rabbit-stalker.prosimcorp.com/v1alpha1
kind: WorkloadAction
metadata:
  name: workloadaction-sample
spec:
  # ...
  condition:
    key: consumers
    value: "0"
```

It's even possible to reach values from inside of arrays. But take care with it, the operator
does not iterate on arrays. Instead, it looks for a specific string to compare the condition. 
For doing the trick on GJSON, it's possible to set conditions in the way it returns a string as follows:

```yaml
apiVersion: rabbit-stalker.prosimcorp.com/v1alpha1
kind: WorkloadAction
metadata:
  name: workloadaction-sample
spec:
  # ...
  condition:
    
    # This will iterate on the array, but will retrieve the first match only
    key: |-
      consumer_details.#(channel_details.node==rabbit@fancy-monk-sample-01).channel_details.node
      
    # which is exactly this, so the condition is met
    value: "rabbit@fancy-monk-sample-01"
```

Anyway, if you decide to get an array, then be sure you fit correctly the value on the condition to match:

```yaml
apiVersion: rabbit-stalker.prosimcorp.com/v1alpha1
kind: WorkloadAction
metadata:
  name: workloadaction-sample
spec:
  # ...
  condition:
    
    # This will return the whole array
    key: |-
      consumer_details.#.channel_details.node

    # Again, is exactly this, so the condition is met
    value: |
      ["rabbit@fancy-monk-sample-01","rabbit@fancy-monk-sample-03"]
```

As a last trick, let's say you need to evaluate a condition where comparing if a number is greater/lower than your value.
By the moment this operation is not supported by the operator, but we plan to add this feature in a future release. 
The awesome point is that supporting gjson for conditions is really helpful, so you can craft an equivalent:

 ```yaml
apiVersion: rabbit-stalker.prosimcorp.com/v1alpha1
kind: WorkloadAction
metadata:
  name: workloadaction-sample
spec:
  # ...
  condition:
    
    # This will return the number ONLY if the number is higher than 8, other way it will return an empty string
    key: |-
      consumers|@values|#(>8)

    # What about comparing an empty string against another empty string? Exactly, you will meet the condition.
    # This means for lower values than 8, the operator will restart the deployment
    value: ""
```

Until now, we have talked about the capabilities of the field `condition.key`, but `condition.value` is really 
powerful too. If `additionalSources` is filled, the content of these sources is available to craft complex values.
All you need to do, is to use the pattern `[<list-index>]{{ <GJSON-expression> }}` inside the `condition.value` field
to use some value coming from a source.

> Hey! Sources is a list composed by `workloadRef` + `additionalSources`. This means position [0] is reserved for 
> the target workload and higher positions starting from [1] will be filled with additionalSources

Let's see an example:

```yaml
apiVersion: rabbit-stalker.prosimcorp.com/v1alpha1
kind: WorkloadAction
metadata:
  name: workloadaction-sample
spec:
  # ...
  condition:
    
    # string literal example
    key: rabbit@fancy-monk-sample-01

    # This will take the value of an annotation named 'node' coming from workloadRef object
    value: "[0]{{ metadata.annotations.node }}"
```

You can craft a value adding some string literals at any side of the pattern used to search. The structure will be 
replaced by the value found in the source:

```yaml
apiVersion: rabbit-stalker.prosimcorp.com/v1alpha1
kind: WorkloadAction
metadata:
  name: workloadaction-sample
spec:
  # ...
  condition:
    
    # string literal example
    key: rabbit@fancy-monk-sample-01

    # This will take the value of an annotation named 'node' coming from the resource in first position at sources list.
    # Imagine the value for this annotation is '1'
    # The '[0]{{ metadata.annotations.node }}' string will be replaced before the comparison, so the final value will 
    # be 'rabbit@fancy-monk-sample-01'
    value: "rabbit@fancy-monk-sample-0[0]{{ metadata.annotations.node }}"
```

As final feature you can use the patterns as many times as needed to build the final string:

```yaml
apiVersion: rabbit-stalker.prosimcorp.com/v1alpha1
kind: WorkloadAction
metadata:
  name: workloadaction-sample
spec:
  # ...
  condition:
    
    # string literal example
    key: rabbit@fancy-monk-sample-03

    # This will take the value from multiples sources in the source list
    # The final value will be 'rabbit@fancy-monk-sample-03' for example
    value: "rabbit@[1]{{ cluster.name }}-0[0]{{ metadata.annotations.node }}"
```

### Running on the cluster

#### Easy way (recommended)

We have designed the deployment of this project to allow remote deployment using Kustomize. This way it is possible
to use it with a GitOps approach, using tools such as ArgoCD or FluxCD. Just make a Kustomization manifest referencing
the tag of the version you want to deploy as follows:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- https://github.com/prosimcorp/rabbit-stalker/releases/download/v1.1.0/bundle.yaml
```

> Notice you can change `v1.1.0` to match some specific release, for example: `v1.x.x`

> 🧚🏼 **Hey, listen! If you prefer to deploy using Helm, go to the [Helm registry](https://github.com/prosimcorp/helm-charts)**

#### Hard way

This way is for passionate learners, but not recommended in production (too manual intervention 🛠️).  

Under the hood, this is executing `kubectl apply -f <some-directories>`

1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=ghcr.io/prosimcorp/rabbit-stalker:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=ghcr.io/prosimcorp/rabbit-stalker:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
This project is done on top of [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder), so read about that project
before collaborating. Of course, we are open to external collaborations for this project. For doing it you must fork the
repository, make your changes to the code and open a PR. The code will be reviewed and tested (always)

> We are developers and hate bad code. For that reason we ask you the highest quality on each line of code to improve
> this project on each iteration.

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## FAQ (Frequently Asked Questions)

### Which API endpoints are supported?
By the moment, only `/api/queues/vhost/name` is supported. This is because it's the most interesting for our use case.
But we will add capabilities to let you choose the endpoint in next releases.

[huge list of RabbitMQ Admin API endpoints](https://rawcdn.githack.com/rabbitmq/rabbitmq-server/v3.11.15/deps/rabbitmq_management/priv/www/api/index.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

