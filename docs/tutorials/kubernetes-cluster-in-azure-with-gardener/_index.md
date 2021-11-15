---
title: Create Shoot Clusters on GCP with Azure
level: beginner
category: Getting Started
scope: app-developer
---

### Overview

### Prerequisites

- You have created an [Azure account](https://azure.microsoft.com/en-us/).
- You have access to the Gardener dashboard and have permissions to create projects.
- You have an Azure Service Principal assigned to your subscription.

### Steps

1. Go to the Gardener dashboard and create a *Project*.

    <img src="images/new-gardener-project.png">


2. Get the properties of your Azure AD tenant, Subscription and Service Principal.
    ```
    Before you can provision and access a Kubernetes cluster on Azure, you need to add the Azure service principal, AD tenant and subscription credentials in Gardener. 
    Gardener needs the credentials to provision and operate the Azure infrastructure for your Kubernetes cluster.

    **Ensure that the Azure service principal has the `Contributor` role within your Subscription assigned.**
    ```


    - Tenant ID
        To find your TenantID, follow this [guide](https://docs.microsoft.com/en-us/azure/active-directory/fundamentals/active-directory-how-to-find-tenant).

    - SubscriptionID
        Select the subscription.
        <img src="images/azureselectsubscription.jpg">

    - Select the Service Principal (SPN).
        <img src="images/azureselectspn.jpg">

    *Note:* A service principal consist of a `ClientID` (also called `ApplicationID`) and a Client Secret. For more information, see [here](https://docs.microsoft.com/en-us/azure/active-directory/develop/app-objects-and-service-principals).
    <img src="images/azuregetclientid.jpg">

    - Client Secret
        Secrets for the Azure Account/Service Principal can be genereted/rotated via the Azure Portal.
        Access the [Azure Portal](https://portal.azure.com) and navigate to the `Active Directory` service.
        Within the service navigate to `App registrations` and select your service principal. 
        In the detail view navigate to `Certificates & secrets`. In the section, you can generate a new secret for the Service Principal.

3. On the Gardener dashboard, choose *Secrets* and then the plus sign <img src="images/plus_icon.jpg"> in the Azure frame to add a new Azure secret.

    <img src="images/gardenernewazuresecret.jpg">

4. Enter the `TenantID`, `SubscriptionID` and the Service Principal credentials (`ClientID` and `ClientSecret`) into the Secret definition.  
    After processing the ticket, you’ll receive the Service Principle credentials via email.
    Copy "Key Value" from the email into "Client Secret".

    <img src="images/gardeneraddazuresecret.jpg">

5. To create a new cluster, choose *Clusters* and then the plus sign in the lower right corner.

    <img src="images/new_cluster.jpg">