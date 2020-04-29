package services

import (
	"clamp-core/models"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"log"
	"net/http"
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
	prefix := fmt.Sprintf("[WORKER_%d] ", workerId)
	prefix = fmt.Sprintf("%15s", prefix)
	log.Printf("%s : Started listening to service request channel\n", prefix)
	for serviceReq := range serviceReqChan {
		executeWorkflow(serviceReq, prefix)
	}
}

func executeWorkflow(serviceReq models.ServiceRequest, prefix string) {
	prefix = fmt.Sprintf("%s [REQUEST_ID: %s]", prefix, serviceReq.ID)
	log.Printf("%s Started processing service request id %s\n", prefix, serviceReq.ID)
	start := time.Now()
	workflow, err := FindWorkflowByName(serviceReq.WorkflowName)
	if err == nil {
		lastStep := workflow.Steps[len(workflow.Steps)-1]
		if serviceReq.CurrentStepId == 0 || serviceReq.CurrentStepId != lastStep.Id {
			status := executeWorkflowSteps(workflow, prefix, serviceReq)
			if status == models.STATUS_COMPLETED {
				elapsed := time.Since(start)
				log.Printf("%s Completed processing service request id %s in %s\n", prefix, serviceReq.ID, elapsed)
			}
		} else {
			log.Printf("%s All steps are executed for service request id: %s\n", prefix, serviceReq.ID)
		}
	}

}

func catchErrors(prefix string, requestId uuid.UUID) {
	if r := recover(); r != nil {
		log.Println("[ERROR]", r)
		log.Printf("%s Failed processing service request id %s\n", prefix, requestId)
	}
}

func executeWorkflowSteps(workflow models.Workflow, prefix string, serviceRequest models.ServiceRequest) models.Status {
	stepRequestPayload := serviceRequest.Payload
	lastStepExecuted := serviceRequest.CurrentStepId
	executeStepsFromIndex := 0
	if lastStepExecuted > 0 {
		executeStepsFromIndex = lastStepExecuted
		log.Printf("%s Skipping steps till  step id %d\n", prefix, executeStepsFromIndex)
	}
	requestContext := CreateRequestContext(workflow, serviceRequest)
	//prepare request context for async steps

	if executeStepsFromIndex > 0 {
		EnhanceRequestContextWithExecutedSteps(&requestContext)
	}

	for i, step := range workflow.Steps[executeStepsFromIndex:] {
		ComputeRequestToCurrentStepInContext(workflow, step, &requestContext, executeStepsFromIndex+i, stepRequestPayload)
		err := ExecuteWorkflowStep(step, requestContext, prefix)
		if !err.IsNil() {
			return models.STATUS_FAILED
		}
		if step.Type == "ASYNC" {
			log.Printf("%s : Pushed to sleep mode until response for step - %s is recieved", prefix, step.Name)
			return models.STATUS_PAUSED
		}
	}
	return models.STATUS_COMPLETED
}

//TODO: replace prefix with other standard way like MDC
func ExecuteWorkflowStep(step models.Step, requestContext models.RequestContext, prefix string) models.ClampErrorResponse {
	serviceRequestId := requestContext.ServiceRequestId
	workflowName := requestContext.WorkflowName
	stepRequest := requestContext.StepsContext[step.Name].Request

	defer catchErrors(prefix, serviceRequestId)

	requestContext.SetStepRequestToContext(step.Name, stepRequest)

	stepStartTime := time.Now()
	stepStatus := models.StepsStatus{
		ServiceRequestId: serviceRequestId,
		WorkflowName:     workflowName,
		StepName:         step.Name,
		Payload: models.Payload{
			Request:  stepRequest,
			Response: nil,
		},
		StepId: step.Id,
	}

	//TODO Condition should be checked on transformed request or original request? Based on that this section needs to be altered
	if step.Transform {
		transform, transformErrors := step.DoTransform(stepRequest, prefix)
		if transformErrors != nil {
			log.Println("Error while transforming request payload")
		}
		requestContext.SetStepRequestToContext(step.Name, transform)
		stepStatus.Payload.Request = transform
	}
	recordStepStartedStatus(stepStatus, stepStartTime)

	resp, err := step.DoExecute(requestContext, prefix)
	if err != nil {
		clampErrorResponse := models.CreateErrorResponse(http.StatusBadRequest, err.Error())
		recordStepFailedStatus(stepStatus, *clampErrorResponse, stepStartTime)
		errFmt := fmt.Errorf("%s Failed executing step %s, %s \n", prefix, stepStatus.StepName, err.Error())
		panic(errFmt)
		return *clampErrorResponse
	} else if step.DidStepExecute() && resp != nil && step.Type == "SYNC" {
		log.Printf("%s Step response received: %s", prefix, resp.(string))
		var responsePayload map[string]interface{}
		json.Unmarshal([]byte(resp.(string)), &responsePayload)
		stepStatus.Payload.Response = responsePayload
		recordStepCompletionStatus(stepStatus, stepStartTime)
		requestContext.SetStepResponseToContext(step.Name, responsePayload)
		return models.EmptyErrorResponse()
	} else if !step.DidStepExecute() {
		//record step skipped
		//setting response of skipped step with same as request for future validations use
		requestContext.SetStepResponseToContext(step.Name, requestContext.GetStepRequestFromContext(step.Name))
		recordStepSkippedStatus(stepStatus, stepStartTime)
		return models.EmptyErrorResponse()
	}
	return models.EmptyErrorResponse()
}

func recordStepCompletionStatus(stepStatus models.StepsStatus, stepStartTime time.Time) {
	stepStatus.Status = models.STATUS_COMPLETED
	stepStatus.TotalTimeInMs = time.Since(stepStartTime).Nanoseconds() / models.MilliSecondsDivisor
	SaveStepStatus(stepStatus)
}

func recordStepSkippedStatus(stepStatus models.StepsStatus, stepStartTime time.Time) {
	stepStatus.Status = models.STATUS_SKIPPED
	stepStatus.TotalTimeInMs = time.Since(stepStartTime).Nanoseconds() / models.MilliSecondsDivisor
	SaveStepStatus(stepStatus)
}

func recordStepPausedStatus(stepStatus models.StepsStatus, stepStartTime time.Time) {
	stepStatus.Status = models.STATUS_PAUSED
	stepStatus.TotalTimeInMs = time.Since(stepStartTime).Nanoseconds() / models.MilliSecondsDivisor
	SaveStepStatus(stepStatus)
}

func recordStepStartedStatus(stepStatus models.StepsStatus, stepStartTime time.Time) {
	stepStatus.Status = models.STATUS_STARTED
	stepStatus.TotalTimeInMs = time.Since(stepStartTime).Nanoseconds() / models.MilliSecondsDivisor
	SaveStepStatus(stepStatus)
}

func recordStepFailedStatus(stepStatus models.StepsStatus, clampErrorResponse models.ClampErrorResponse, stepStartTime time.Time) {
	stepStatus.Status = models.STATUS_FAILED
	marshal, marshalErr := json.Marshal(clampErrorResponse)
	log.Println("clampErrorResponse: Marshal error", marshalErr)
	var responsePayload map[string]interface{}
	unmarshalErr := json.Unmarshal(marshal, &responsePayload)
	log.Println("clampErrorResponse: UnMarshal error", unmarshalErr)
	errPayload := map[string]interface{}{"errors": responsePayload}
	stepStatus.Payload.Response = errPayload
	stepStatus.Reason = clampErrorResponse.Message
	stepStatus.TotalTimeInMs = time.Since(stepStartTime).Nanoseconds() / models.MilliSecondsDivisor
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
