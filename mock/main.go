package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func handleSlothfulMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	// Simulating request latency
	time.Sleep(2 * time.Second)
	fmt.Println("Latency is gone... Shot the request")

	//
	w.Write([]byte(`
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
    }`))
}

func appRouter() http.Handler {
	rt := http.NewServeMux()
	rt.HandleFunc("/api/queues/shared/test", handleSlothfulMessage)
	return rt
}

func main() {

	log.Print("This is a mock server for Rabbit Stalker testing purposes")
	log.Print("To expose this server over Internet, execute the following: ssh -R 80:localhost:8090 nokey@localhost.run")

	err := http.ListenAndServe(":8090", appRouter())
	if err != nil {
		panic("Error: " + err.Error())
	}

	log.Print("Server has been closed. Start it again")
}
