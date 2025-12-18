kubectl create namespace nifi

там где nifi.k8c.ru - заменить на свой домен

keycloak.___.ru - ваш keycloak 

openssl s_client -connect keycloak.___.ru:443 -servername keycloak.___.ru -showcerts </dev/null 2>/dev/null | sed -n '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p' | kubectl create secret generic keycloak-nifi-secret --from-file=tls.crt=/dev/stdin -n nifi

kubectl apply -f nifi-tls-secret.yaml -n nifi

kubectl apply -f certificate-api.yaml -n nifi
kubectl wait --for=condition=Ready certificate/nifi-admin-gateway-cert -n nifi --timeout=300s

# Однострочник с автоматическим удалением
kubectl apply -f nifi-cert-generator-job.yaml -n nifi && \
kubectl wait --for=condition=complete job/nifi-cert-generator -n nifi --timeout=300s && \
kubectl delete job nifi-cert-generator -n nifi

kubectl get cert -n nifi - дождаться true
__________________________________________________________

kubectl apply -f nifi-operator-deployment-v4.yaml

kubectl apply -f nifi.yaml -n nifi


openssl s_client -connect keyc.kube5s.ru:443 -servername keyc.kube5s.ru -showcerts </dev/null 2>/dev/null | sed -n '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p' | kubectl create secret generic keycloak-nifi-secret --from-file=tls.crt=/dev/stdin -n nifi