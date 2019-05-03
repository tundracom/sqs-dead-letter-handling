# SQS Dead Letter Handling

Binaries for handling SQS Dead Letter Queues:

* sqs-dead-letter-requeue: Requeue all messages from dead letter queue to related active queue
* maintains the FIFO order, regardless of the queue type (standard, FIFO), oldest messages are being re-queued first

## Requirements

* Golang

## Building it

### sqs-dead-letter-requeue
```sh
go build -o bin/sqs-dead-letter-requeue sqs-dead-letter-requeue/main.go
```

## Running it

Make sure you have the environment variables for AWS set

```sh
export AWS_ACCESS_KEY_ID=<my-access-key>
export AWS_SECRET_ACCESS_KEY=<my-secret-key>
```

### sqs-dead-letter-requeue
```sh
bin/sqs-dead-letter-requeue --source-queue-name=buy-worker-dead-letter buy-worker

# only one message
bin/sqs-dead-letter-requeue --messages=1 --source-queue-name=buy-worker-dead-letter buy-worker
```

### Send test messages on the dead letter queue
```sh
# reindex a store
aws sqs send-message --queue-url=https://eu-west-1.queue.amazonaws.com/429416768433/buy-worker-dead-letter --message-body='{"storeUri":"velo"}' --message-attributes='{ "Type": {"DataType": "String", "StringValue": "vox.buy.worker.controller.search.ReindexStoreMessage"} }'
```

```sh
# set shipment status to verified
aws sqs send-message --queue-url=https://eu-west-1.queue.amazonaws.com/429416768433/shipment-execution-dead-letter.fifo --message-body='{"shipmentId":"2160693-01","userId":"2150868","event":{"_":"vox.buy.model.TundraEvent$Verify"}}' --message-attributes='{ "Type": {"DataType": "String", "StringValue": "vox.buy.model.ShipmentEventMessage"} }' --message-group-id=2
```
