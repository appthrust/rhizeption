# Rhizeption

Rhizeption is an innovative project that implements Hosted Nodes for Kubernetes clusters, providing a lightweight, efficient, and highly scalable solution for container orchestration.

## Table of Contents

- [Introduction](#introduction)
- [Hosted Nodes: In-Depth](#hosted-nodes-in-depth)
- [Key Features](#key-features)
- [Architecture](#architecture)
- [Getting Started](#getting-started)
- [Contributing](#contributing)
- [License](#license)

## Introduction

Rhizeption extends the Kubernetes ecosystem by introducing the concept of Hosted Nodes. These nodes are containerized Kubernetes nodes that run within a host Kubernetes cluster, offering a nested and interconnected structure that maximizes resource utilization and operational flexibility.

## Hosted Nodes: In-Depth

Hosted Nodes represent a paradigm shift in how we think about and implement Kubernetes clusters. Here's a detailed look at what Hosted Nodes are and how they work:

### What are Hosted Nodes?

Hosted Nodes are Kubernetes nodes that run as containers within a host Kubernetes cluster. This nested structure allows for a more efficient use of resources and provides greater flexibility in cluster management.

### How do Hosted Nodes differ from traditional Kubernetes nodes?

1. **Resource Efficiency**: Traditional Kubernetes nodes typically run on virtual machines or bare metal servers. Hosted Nodes, being containers themselves, have significantly lower overhead, allowing for higher density and better resource utilization.

2. **Rapid Provisioning**: Hosted Nodes can be created and destroyed much faster than traditional VMs, enabling quicker scaling and more agile cluster management.

3. **Nested Architecture**: Hosted Nodes create a cluster-within-a-cluster structure, allowing for innovative approaches to multi-tenancy, isolation, and resource partitioning.

4. **Simplified Management**: By leveraging container orchestration for node management, Hosted Nodes can be managed using familiar Kubernetes primitives and tools.

### Technical Implementation

Rhizeption implements Hosted Nodes using the following components:

1. **HnCluster**: A custom resource that defines the overall configuration for a cluster of Hosted Nodes.

2. **HnMachine**: A custom resource representing individual Hosted Nodes. Each HnMachine corresponds to a Pod in the host cluster, running a containerized Kubernetes node.

3. **Custom Controllers**: Rhizeption includes controllers that manage the lifecycle of HnClusters and HnMachines, ensuring they stay in sync with the desired state.

4. **Containerd**: Used as the container runtime within Hosted Nodes, providing a lightweight and efficient container execution environment.

5. **Networking**: Implements advanced networking solutions to ensure proper communication between Hosted Nodes, as well as between Hosted Nodes and the host cluster.

### Use Cases

Hosted Nodes are particularly useful in scenarios such as:

1. **Development and Testing**: Quickly spin up and tear down full Kubernetes environments for testing and development purposes.

2. **Multi-Tenancy**: Create isolated Kubernetes environments for different teams or customers within a single host cluster.

3. **Edge Computing**: Deploy lightweight Kubernetes nodes in resource-constrained edge environments.

4. **Cost Optimization**: Maximize resource utilization in cloud environments by running multiple Kubernetes clusters on a single set of VMs.

5. **Learning and Education**: Provide individual Kubernetes environments to learners without the need for separate VMs or physical hardware.

## Key Features

- **Lightweight Nodes**: Rhizeption nodes are containerized, resulting in reduced resource overhead compared to traditional VMs.
- **Nested Architecture**: Supports running Kubernetes nodes within a Kubernetes cluster, enabling complex and flexible deployments.
- **Standard Kubernetes Compatibility**: Fully compatible with standard Kubernetes control planes and tools.
- **Efficient Resource Utilization**: Maximizes cluster resource usage through containerized node management.
- **Rapid Scaling**: Quickly scale your cluster up or down by adding or removing containerized nodes.
- **Flexible Networking**: Implements advanced networking capabilities to ensure smooth communication between hosted nodes and the host cluster.

## Architecture

Rhizeption consists of two main custom resources:

1. **HnCluster**: Manages the overall hosted node cluster configuration and lifecycle. It defines cluster-wide settings such as networking configuration and control plane endpoints.

2. **HnMachine**: Represents individual hosted nodes within the cluster. Each HnMachine is realized as a Pod running a containerized Kubernetes node.

These custom resources extend the Cluster API, allowing seamless integration with existing Kubernetes workflows and tools.

The project uses containerd as the container runtime and implements custom controllers to manage the lifecycle of hosted nodes. The controllers ensure that the actual state of the Hosted Nodes cluster matches the desired state defined in the HnCluster and HnMachine resources.

## Getting Started

### Prerequisites

- Kubernetes cluster (v1.28.3 or later)
- kubectl configured to communicate with your cluster
- Cluster API components installed on your cluster

### Installation

1. Clone the Rhizeption repository:
   ```
   git clone https://github.com/your-org/rhizeption.git
   cd rhizeption
   ```

2. Install the Custom Resource Definitions (CRDs):
   ```
   kubectl apply -f config/crd/bases
   ```

3. Deploy the Rhizeption controllers:
   ```
   kubectl apply -f config/manager/manager.yaml
   ```

4. Create your first HnCluster:
   ```
   kubectl apply -f config/samples/hn_v1alpha1_hncluster.yaml
   ```

5. Verify the installation:
   ```
   kubectl get hnclusters
   kubectl get hnmachines
   ```

For more detailed instructions and advanced configurations, please refer to our [documentation](https://docs.rhizeption.io).

## Contributing

We welcome contributions to Rhizeption! Please read our [Contributing Guide](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## License

Rhizeption is released under the [Apache 2.0 License](LICENSE).