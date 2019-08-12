package main // import "github.com/tundradotcom/sqs-dead-letter-requeue"

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"gopkg.in/alecthomas/kingpin.v1"
	"log"
	"os"
	"sort"
	"strconv"
)

var (
	app           = kingpin.New("dead-letter-requeue", "Requeue messages from a SQS dead-letter queue to the active one.")
	queueName     = app.Arg("destination-queue-name", "Name of the destination SQS queue (e.g. buy-worker).").Required().String()
	fromQueueName = app.Flag("source-queue-name", "Name of the source SQS queue (e.g. buy-worker-dead-letter).").String()
	accountID     = app.Flag("account-id", "AWS account ID. (e.g. 123456789)").String()
	messageLimit  = app.Flag("messages", "messages to process").Int()
)

type byTimestamp []*sqs.Message

func extractAwsTimestamp(m *sqs.Message) int64 {
	ts := m.Attributes["SentTimestamp"]
	i, err := strconv.ParseInt(*ts, 10, 64)
	if err != nil {
		panic(err)
	}
	return i
}

func (a byTimestamp) Len() int           { return len(a) }
func (a byTimestamp) Less(i, j int) bool { return extractAwsTimestamp(a[i]) < extractAwsTimestamp(a[j]) }
func (a byTimestamp) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func getQueueUrlnput(queueName *string, accountID *string) *sqs.GetQueueUrlInput {
	var getQueueURLInput sqs.GetQueueUrlInput
	if *accountID != "" {
		getQueueURLInput = sqs.GetQueueUrlInput{QueueName: queueName, QueueOwnerAWSAccountId: accountID}
	} else {
		getQueueURLInput = sqs.GetQueueUrlInput{QueueName: queueName}
	}
	return &getQueueURLInput
}

// collect all the messages from the queue, relying on the visibility timeout
// IMPORTANT the sequence is not ordered!
func readQueue(conn *sqs.SQS, sourceQueueURL *sqs.GetQueueUrlOutput, messageLimit int) []*sqs.Message {
	var messages []*sqs.Message

	waitTimeSeconds := int64(20)
	maxNumberOfMessages := int64(10)
	// in seconds, set to a minute, then won't appear in the loop
	visibilityTimeout := int64(60)

	for i := 0; i < messageLimit; {
		resp, err := conn.ReceiveMessage(&sqs.ReceiveMessageInput{
			AttributeNames:        aws.StringSlice([]string{"All"}),
			MessageAttributeNames: aws.StringSlice([]string{"All"}),
			WaitTimeSeconds:       &waitTimeSeconds,
			MaxNumberOfMessages:   &maxNumberOfMessages,
			VisibilityTimeout:     &visibilityTimeout,
			QueueUrl:              sourceQueueURL.QueueUrl})

		if err != nil {
			log.Fatal(err)
			// return with empty array of messages in case of failure
			return []*sqs.Message{}
		}

		numberOfMessagesRead := len(resp.Messages)
		if numberOfMessagesRead == 0 {
			return messages
		}

		log.Printf("Collecting %v messages", numberOfMessagesRead)
		messages = append(messages, resp.Messages...)

		i += numberOfMessagesRead
	}
	return messages
}

func requeueMessage(conn *sqs.SQS, sourceQueueURL *sqs.GetQueueUrlOutput, destinationQueueURL *sqs.GetQueueUrlOutput, message *sqs.Message) {
	respAdd, errAdd := conn.SendMessage(&sqs.SendMessageInput{
		MessageAttributes: message.MessageAttributes,
		MessageBody:       message.Body,
		MessageGroupId:    message.Attributes["MessageGroupId"],
		QueueUrl:          destinationQueueURL.QueueUrl})
	if errAdd != nil {
		panic(errAdd)
	}
	log.Printf("Requeued message [%s] -> [%s] to [%s]", *message.MessageId, *respAdd.MessageId, *destinationQueueURL.QueueUrl)

	// remove it from the source queue
	_, errDel := conn.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      sourceQueueURL.QueueUrl,
		ReceiptHandle: message.ReceiptHandle,
	})
	if errDel != nil {
		log.Fatal(errDel)
	} else {
		log.Printf("Removed message [%s] from [%s]", *message.MessageId, *sourceQueueURL.QueueUrl)
	}
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	destinationQueueName := *queueName
	var sourceQueueName string

	if *fromQueueName != "" {
		sourceQueueName = *fromQueueName
	} else {
		sourceQueueName = destinationQueueName + "-dead-letter"
	}

	if messageLimit == nil || *messageLimit == 0 {
		*messageLimit = 100
	}
	log.Printf("Source queue [%s] ", sourceQueueName)
	log.Printf("Destination queue [%s] ", destinationQueueName)
	log.Printf("Messages to process [%v] ", *messageLimit)

	sess, err := session.NewSession()
	if err != nil {
		log.Fatal(err)
		return
	}
	conn := sqs.New(sess)

	sourceQueueURL, err := conn.GetQueueUrl(getQueueUrlnput(&sourceQueueName, accountID))
	if err != nil {
		log.Fatal(err)
		return
	}
	destinationQueueURL, err := conn.GetQueueUrl(getQueueUrlnput(&destinationQueueName, accountID))
	if err != nil {
		log.Fatal(err)
		return
	}

	log.Printf("Looking for messages to requeue.")
	messages := readQueue(conn, sourceQueueURL, *messageLimit)
	numberOfMessages := len(messages)
	log.Printf("Found [%v] messages to requeue", numberOfMessages)

	// sort by order of arrival
	sort.Sort(byTimestamp(messages))

	for index, element := range messages {
		log.Printf("Message[%v] [%s]", index, *element.MessageId)
	}

	for index, element := range messages {
		if index >= *messageLimit {
			log.Printf("\n_____________________________\nProcessed %v messages", *messageLimit)
			return
		}
		log.Println()
		messageType := "unknown"
		if element.MessageAttributes["Type"] != nil {
			messageType = *element.MessageAttributes["Type"].StringValue
		}
		log.Printf("Requeue message[%v] [%s] type -> %s", index, *element.MessageId, messageType)
		requeueMessage(conn, sourceQueueURL, destinationQueueURL, element)
		log.Printf("Requeue succeeded for message[%v] [%s]", index, *element.MessageId)
	}
}
