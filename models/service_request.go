package models

import (
	"github.com/google/uuid"
	"time"
)

//ServiceRequest is a structure to store the service request details
type ServiceRequest struct {
	ID           uuid.UUID              `json:"id"`
	WorkflowName string                 `json:"workflow_name"`
	Status       Status                 `json:"status"`
	CreatedAt    time.Time              `json:"created_at"`
	Payload      map[string]interface{} `json:"payload"`
	//TODO: rename to last step id executed
	CurrentStepId int `json:"current_step_id",binding:"omitempty"`
}

type Status string

const (
	STATUS_NEW        Status = "NEW"
	STATUS_STARTED    Status = "STARTED"
	STATUS_RESUMED    Status = "RESUMED"
	STATUS_PAUSED     Status = "PAUSED"
	STATUS_COMPLETED  Status = "COMPLETED"
	STATUS_FAILED     Status = "FAILED"
	STATUS_INPROGRESS Status = "IN_PROGRESS"
)

func NewServiceRequest(workflowName string, payload map[string]interface{}) ServiceRequest {
	currentTime := time.Now()
	return ServiceRequest{ID: uuid.New(), WorkflowName: workflowName, Status: STATUS_NEW, CreatedAt: currentTime, Payload: payload}
}

type PGServiceRequest struct {
	tableName    struct{} `pg:"service_requests"`
	ID           uuid.UUID
	WorkflowName string
	Status       Status
	CreatedAt    time.Time
	Payload      map[string]interface{} `json:"payload"`
}

func (serviceReq ServiceRequest) ToPgServiceRequest() PGServiceRequest {
	return PGServiceRequest{
		ID:           serviceReq.ID,
		WorkflowName: serviceReq.WorkflowName,
		Status:       serviceReq.Status,
		CreatedAt:    serviceReq.CreatedAt,
		Payload:      serviceReq.Payload,
	}
}

func (pgServReq PGServiceRequest) ToServiceRequest() ServiceRequest {
	return ServiceRequest{
		ID:           pgServReq.ID,
		WorkflowName: pgServReq.WorkflowName,
		Status:       pgServReq.Status,
		CreatedAt:    pgServReq.CreatedAt,
		Payload:      pgServReq.Payload,
	}
}