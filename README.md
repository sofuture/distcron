## Overview
A comprehensive discussion on distributed job schedulers is available from 
[Google](https://queue.acm.org/detail.cfm?id=2745840) 
and [Facebook](https://facebook.github.io/bistro/static/bistro_ATC_final.pdf) we will borrow most design decisions from there. 

## Clustering
### Membership 
Hosts join the cluster and maintain its membership based on gossip protocol. 
[See Serf](https://github.com/hashicorp/serf). A node will need to be aware about any other node 
in order to join the cluster. Maintaining list of previously known peers 
and other auto discovery mechanisms such as provided by GCP and AWS environments are left out of scope for this implementation.

## Leader
Leader election is based on acquiring distributed lock over KV storage, see github.com/docker/leadership

## Followers
When node is in Follower state, it will only be able to accept commands from Leader to run a specific task.

## Jobs & Schedule
The usecase of distributed job scheduler will unlikely be limited to running a typical local CRON tasks doing local host housekeeping, 
but likely more complex workloads users expect to be elastically executed across available infrastructure.

In such scenarios, distributing and updating software including its dependencies will likely be a major issue, 
and using pre-built Docker containers a more natural choice rather then just supplying an executable and custom files to run. 

By using Docker, we can [limit container resources](https://docs.docker.com/engine/admin/resource_constraints/) 
without need to create cgroups and namespaces manually. 

Use of Docker also provides path towards [live migration](https://github.com/docker/docker/blob/master/experimental/checkpoint-restore.md) support. 

### Output Logging
For production environment, forwarding logs from individual Docker container run 
to centralized location by means of custom logging driver might be a solution of choice. 
Browsing historical or real-time feed would be a matter of filtering logs by specific job ID then.

In our simplified case, we will just bind output of docker logs command with HTTP API in order to provide live view to remote observer.

### Scheduler
Every job is defined by set of runtime parameters, and execution constraints. The scheduler main loop will: 

* Calculate next time a job should be run
* Notify followers that certain job is about to launch, and update its next scheduled run time
* Selects next available node (see below)
* Spawns process on remote node
* Notify followers that job is now running on certain node 
* Remote node should notify current leader on job completion, after that Leader notifies followers on job completion

Followers notification is done via Raft, and is strong consistency operation. 

### Concurrent execution
Distributed CRON will spawn jobs when time comes. In case it might be needed to maintain strictly single instance of a job, 
one may implement a distributed lock using i.e. etcd or Raft to prevent certain jobs executing concurrently, similar to flock usage with local crond.

### Node selection
Nodes will continuously provide update on their available resources, such as CPU, RAM and DISK via gossip protocol. 
Scheduler will maintain a state table of nodes, and will apply a naive algorithm to pick machine with most (at least 25%) available resources to execute next workload.
For simplicity, it will not try to estimate whether target host is a best fit for the given workload. If no such machine is found for the task, the task will fail. 

## Configuration and API

* Only Leader will serve API requests and update configuration via Raft, using flat files for internal configuration storage, in order to minimize external dependency requirements (i.e. external database). 
* If API request arrives to a node which is currently in Follower state, it will issue an HTTP 302 (temporary redirect). In production environment it might transparently proxy the request torwards the Leader host.
* As part of Leader node election we may also enable dynamic API discovery features, such as automatic registration of Leader IP address within a dynamic DNS service or registration on external HTTPS load balancer. 

## Configuration 

## Failure and Recovery
In the event of Leader failure, Raft protocol provides a mechanism to elect a new Leader, along with state replication and snapshotting mechanisms in order to keep it up to date. 

The following situations may happen in our current design: (incomplete)
* Master declares a certain job as 'about to run' but fails to actually run it

## Authentication and Security
For simplicity, no authentication and transport layer security will be in place for this implementation. 
