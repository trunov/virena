# virena
github.com/trunov/virena using Go, cars parts shop

Application can be utilized via k8s.

* in order to create secret for k8s:
`kubectl create secret generic virena-secrets --from-literal=DATABASE_URI="your_database_uri" --from-literal=SENDGRID_API_KEY="your_sendgrid_api_key"`

* in order to create port configmap:
`kubectl create configmap virena-config --from-literal=PORT=8080`


* image building and pushing:
`docker build -t virena-golang:latest .`
`docker tag virena-golang:latest {username}/virena-golang:{version}`
`docker push {username}/virena-golang:{version}`