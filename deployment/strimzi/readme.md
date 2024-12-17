### Create namespace
```sh
kubectl create namespace kafka
```

### Add & update repositories (helm)
```sh
helm repo add strimzi https://strimzi.io/charts/
helm repo update
```

### Install crd's [custom resources]
```sh
helm install kafka strimzi/strimzi-kafka-operator --namespace kafka --version 0.38.0
```

### Install kafka-cluster
```sh
kubectl apply -f deployment/strimzi/kafka-cluster.yaml -n kafka
```

### Install kafka-user
```sh
kubectl apply -f deployment/strimzi/kafka-admin.yaml -n kafka
```

### Install kafka-ui
```sh
kubectl apply -f deployment/strimzi/kafka-ui.yaml -n kafka
```