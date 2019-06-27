all: build push

build:
	go get "github.com/aws/aws-sdk-go/aws"
	go get "github.com/aws/aws-sdk-go/aws/session"
	go get "github.com/aws/aws-sdk-go/service/sqs"
	go get "gopkg.in/alecthomas/kingpin.v1"
	docker run --rm -v "$(CURDIR)/sqs-dead-letter-requeue:/src" -v /var/run/docker.sock:/var/run/docker.sock centurylink/golang-builder tundradotcom/sqs-dead-letter-requeue

push:
	docker push tundradotcom/sqs-dead-letter-requeue:latest
