#!/bin/bash

SERVICE=podpreset-service
NAMESPACE=podpreset-crd-system

cfssl gencert -initca ca-csr.json | cfssljson -bare ca
cfssl gencert \
  -ca=ca.pem \
  -ca-key=ca-key.pem \
  -config=ca-config.json \
  -hostname=127.0.0.1,$SERVICE,$SERVICE.kube-system,$SERVICE.$NAMESPACE,$SERVICE.$NAMESPACE.svc \
  -profile=default \
  webhook-csr.json | cfssljson -bare $SERVICE

kubectl create secret tls $SERVICE-tls -n $NAMESPACE \
  --cert=$SERVICE.pem \
  --key=$SERVICE-key.pem

base64 -w 0 ca.pem > ca.pem.base64
echo "Updating deployment caBundle"
sed -i -E "s/(caBundle: )(.*)/\1$(cat ca.pem.base64)/" ../deployment.yaml
