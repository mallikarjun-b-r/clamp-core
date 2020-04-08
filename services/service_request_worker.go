package services

import (
	"clamp-core/models"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"log"
	"sync"
	"time"
)

const ServiceRequestChannelSize = 1000
const ServiceRequestWorkersSize = 100

var (
	serviceRequestChannel chan models.ServiceRequest
	singletonOnce         sync.Once
)

func createServiceRequestChannel() chan models.ServiceRequest {
	singletonOnce.Do(func() {
		serviceRequestChannel = make(chan models.ServiceRequest, ServiceRequestChannelSize)
	})
	return serviceRequestChannel
}

func init() {
	createServiceRequestChannel()
	createServiceRequestWorkers()
}

func createServiceRequestWorkers() {
	for i := 0; i < ServiceRequestWorkersSize; i++ {
		go worker(i, serviceRequestChannel)
	}
}

func worker(workerId int, serviceReqChan <-chan models.ServiceRequest) {
	prefix := fmt.Sprintf("[WORKER_%d] : ", workerId)
	prefix = fmt.Sprintf("%15s", prefix)
	log.Printf("%s Started listening to service request channel\n", prefix)
	for serviceReq := range serviceReqChan {
		executeWorkflow(serviceReq, prefix)
	}
}

func executeWorkflow(serviceReq models.ServiceRequest, prefix string) {
	prefix = prefix[:len(prefix)-2] + fmt.Sprintf("[REQUEST_ID: %s]", serviceReq.ID)
	log.Printf("%s Started processing service request id %s\n", prefix, serviceReq.ID)
	var stepStatus models.StepsStatus
	defer catchErrors(prefix, serviceReq.ID)
	stepStatus.ServiceRequestId = serviceReq.ID
	stepStatus.WorkflowName = serviceReq.WorkflowName
	stepStatus.Payload = serviceReq.Payload

	start := time.Now()
	workflow, err := FindWorkflowByName(serviceReq.WorkflowName)
	if err == nil && workflow != nil {
		executeWorkflowStepsInSync(workflow, prefix, stepStatus)
	}
	elapsed := time.Since(start)
	log.Printf("%s Completed processing service request id %s in %s\n", prefix, serviceReq.ID, elapsed)
}

func catchErrors(prefix string, requestId uuid.UUID) {
	if r := recover(); r != nil {
		log.Println("[ERROR]", r)
		log.Printf("%s Failed processing service request id %s\n", prefix, requestId)
	}
}

func executeWorkflowStepsInSync(workflow *models.Workflow, prefix string, stepStatus models.StepsStatus) {
	for _, step := range workflow.Steps {
		stepStartTime := time.Now()
		log.Printf("%s Started executing step id %s\n", prefix, step.Id)
		stepStatus.StepName = step.Name
		recordStepStartedStatus(stepStatus, stepStartTime)
		oldPrefix := log.Prefix()
		log.SetPrefix(oldPrefix + prefix)
		resp, err := step.DoExecute()
		log.SetPrefix(oldPrefix)
		if err != nil {
			recordStepFailedStatus(stepStatus, err, stepStartTime, prefix)
			errFmt := fmt.Errorf("%s Failed executing step %s, %s \n", prefix, stepStatus.StepName, err.Error())
			panic(errFmt)
		}
		if resp != nil {
			log.Printf("%s Received %s", prefix, resp.(string))
			recordStepCompletionStatus(stepStatus, stepStartTime)
		}
	}
}

func recordStepCompletionStatus(stepStatus models.StepsStatus, stepStartTime time.Time) {
	stepStatus.Status = models.STATUS_COMPLETED
	stepStatus.TotalTimeInMs = time.Since(stepStartTime).Nanoseconds() / 1000
	SaveStepStatus(stepStatus)
}

func recordStepStartedStatus(stepStatus models.StepsStatus, stepStartTime time.Time) {
	stepStatus.Status = models.STATUS_STARTED
	stepStatus.TotalTimeInMs = time.Since(stepStartTime).Nanoseconds() / 1000
	SaveStepStatus(stepStatus)
}

func recordStepFailedStatus(stepStatus models.StepsStatus, err error, stepStartTime time.Time, prefix string) {
	stepStatus.Status = models.STATUS_FAILED
	stepStatus.Reason = err.Error()
	stepStatus.TotalTimeInMs = time.Since(stepStartTime).Nanoseconds() / 1000
	SaveStepStatus(stepStatus)
}

func getServiceRequestChannel() chan models.ServiceRequest {
	if serviceRequestChannel == nil {
		panic(errors.New("service request channel not initialized"))
	}
	return serviceRequestChannel
}

func AddServiceRequestToChannel(serviceReq models.ServiceRequest) {
	channel := getServiceRequestChannel()
	channel <- serviceReq
}
