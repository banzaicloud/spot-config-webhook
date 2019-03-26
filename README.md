![license](http://img.shields.io/badge/license-Apache%20v2-orange.svg)
# Spot config webhook

> This webhook is part of the [Pipeline](https://github.com/banzaicloud/pipeline) platform. It can be used independently but it was designed to work effectively with Pipeline.

The [Pipeline](https://github.com/banzaicloud/pipeline) platform is using mutating webhooks to set a *custom scheduler* on deployments when the cluster have spot instances and when Pipeline signals that the deployment should have at least some percent of replicas placed on on-demand instances.

When creating a deployment through the Pipeline API or from the UI the user can specify how many percentage of her workload must run on safe, on-demand instances.
After the API request is sent, Pipeline updates a `ConfigMap` that stores this information about various deployments and creates the deployment itself through Helm.

When the deployment request is sent to the `apiserver`, this webhook intercepts the request, and checks if the deployment is present in the `ConfigMap`.
If it's found, the webhook mutates the request to include a special annotation that can be parsed by the spot-affinity scheduler, and modifies the `schedulerName` in the pod template `spec`.

### Deploying the webhook

This webhook is automatically deployed to any cluster started with Pipeline that has spot instances, but it can be deployed independently using the same Helm chart what Pipeline is using.
The chart creates the necessary `MutatingWebhookConfiguration` as well as the resources - like the TLS certifications - needed by the `ApiService` extension to run in an RBAC-enabled cluster.
The chart is available in the `banzaicloud-stable` repo:

```
helm repo add banzaicloud-stable http://kubernetes-charts.banzaicloud.com/branch/master
helm repo update
```

To install run the following command:
```
helm install --name <name> banzaicloud-stable/spot-config-webhook
```



