# Migrate Azure Shoot Load Balancer from basic to standard SKU

This guide descibes how to migrate the Load Balancer of an Azure Shoot cluster from the basic SKU to the standard SKU.<br/>
**Be aware:** You need to delete and recreate all services of type Load Balancer, which means that the public ip addresses of your service endpoints will change.<br/>
Please do this only if the Stakeholder really needs to migrate this Shoot to use standard Load Balancers. All new Shoot clusters will automatically use Azure Standard Load Balancers.

1. Disable temporarily Gardeners reconciliation.<br>
The Gardener Controller Manager need to be configured to allow ignoring Shoot clusters.
This can be configured in its the `ControllerManagerConfiguration` via the field `.controllers.shoot.respectSyncPeriodOverwrite="true"`.

```sh
# In the Garden cluster.
kubectl annotate shoot <shoot-name> shoot.garden.sapcloud.io/ignore="true"

# In the Seed cluster.
kubectl -n <shoot-namespace> scale deployment gardener-resource-manager --replicas=0
```

2. Backup all Kubernetes services of type Load Balancer.
```sh
# In the Shoot cluster.
# Determine all Load Balancer services.
kubectl get service --all-namespaces | grep LoadBalancer

# Backup each Load Balancer service.
echo "---" >> service-backup.yaml && kubectl -n <namespace> get service <service-name> -o yaml >> service-backup.yaml
```

3. Delete all Load Balancer services.
```sh
# In the Shoot cluster.
kubectl -n <namespace> delete service <service-name>
```

4. Wait until until Load Balancer is deleted.
Wait until all services of type Load Balancer are deleted and the Azure Load Balancer resource is also deleted.
Check via the Azure Portal if the Load Balancer within the Shoot Resource Group has been deleted.
This should happen automatically after all Kubernetes Load Balancer service are gone within a few minutes.

Alternatively the Azure cli can be used to check the Load Balancer in the Shoot Resource Group.
The credentials to configure the cli are available on the Seed cluster in the Shoot namespace.
```sh
# In the Seed cluster.
# Fetch the credentials from cloudprovider secret.
kubectl -n <shoot-namespace> get secret cloudprovider -o yaml

# Configure the Azure cli, with the base64 decoded values of the cloudprovider secret.
az login --service-principal --username <clientID> --password <clientSecret> --tenant <tenantID>
az account set -s <subscriptionID>

# Fetch the constantly the Shoot Load Balancer in the Shoot Resource Group. Wait until the resource is gone.
watch 'az network lb show -g shoot--<project-name>--<shoot-name> -n shoot--<project-name>--<shoot-name>'

# Logout.
az logout
```

5. Modify the `cloud-povider-config` configmap in the Seed namespace of the Shoot.<br/>
The key `cloudprovider.conf` contains the Kubernetes cloud-provider configuration.
The value is a multiline string. Please change the value of the field `loadBalancerSku` from `basic` to `standard`.
Iff the field does not exists then append `loadBalancerSku: \"standard\"\n` to the value/string.
```sh
# In the Seed cluster.
kubectl -n <shoot-namespace> edit cm cloud-provider-config
```

6. Enable Gardeners reconcilation and trigger a reconciliation.
```
# In the Garden cluster
# Enable reconcilation
kubectl annotate shoot <shoot-name> shoot.garden.sapcloud.io/ignore-

# Trigger reconcilation
kubectl annotate shoot <shoot-name> shoot.garden.sapcloud.io/operation="reconcile"
```
Wait until the cluster has been reconciled.

6. Recreate the services from the backup file.<br/>
Probably you need to remove some fields from the service defintions e.g. `.spec.clusterIP`, `.metadata.uid` or `.status` etc.
```sh
kubectl apply -f service-backup.yaml
```

7. If successful remove backup file.
```sh
# Delete the backup file.
rm -f service-backup.yaml
```

