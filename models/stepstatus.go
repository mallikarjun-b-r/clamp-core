package models

import (
	"github.com/google/uuid"
	"time"
)

//Step Status is a structure to store the service request steps details
type StepStatus struct {
	ID           			   string `json:"id"`
	ServiceRequestId           uuid.UUID `json:"serviceRequestId"`
	WorkflowName 			   string    `json:"workflowName"`
	Status       			   Status    `json:"status"`
	CreatedAt    			   time.Time    `json:"createdAt"`
	StepName   				   string   `json:"stepName"`
	Reason   				   string    `json:"reason"`
}

func NewStepStatus(stepStatus StepStatus) StepStatus {
	return StepStatus{ID: stepStatus.ID, ServiceRequestId:stepStatus.ServiceRequestId,WorkflowName: stepStatus.WorkflowName,
		Status: STATUS_STARTED, CreatedAt:time.Now(), StepName:stepStatus.StepName, Reason:stepStatus.Reason}
}

//Create a Step Status Entry for a given service request id and return step status details
func CreateStepStatus(stepStatus StepStatus) StepStatus {
	return NewStepStatus(stepStatus)
}

type PGStepStatus struct {
	tableName    struct{} `pg:"steps_status"`
	ID           			   string
	ServiceRequestId           uuid.UUID
	WorkflowName 			   string
	Status       			   Status
	CreatedAt    			   time.Time
	StepName   				   string
	Reason   				   string
}

func (stepStatus StepStatus) ToPgStepStatus() PGStepStatus {
	return PGStepStatus{
		ID:           	  stepStatus.ID,
		ServiceRequestId: stepStatus.ServiceRequestId,
		WorkflowName:     stepStatus.WorkflowName,
		Status:       	  stepStatus.Status,
		CreatedAt:    	  stepStatus.CreatedAt,
		StepName:		  stepStatus.StepName,
		Reason:		      stepStatus.Reason,
	}
}

func (pgStepStatus PGStepStatus) toStepStatus() StepStatus {
	return StepStatus{
		ID:               pgStepStatus.ID,
		ServiceRequestId: pgStepStatus.ServiceRequestId,
		WorkflowName:     pgStepStatus.WorkflowName,
		Status:           pgStepStatus.Status,
		CreatedAt:        pgStepStatus.CreatedAt,
		StepName:         pgStepStatus.StepName,
		Reason:           pgStepStatus.Reason,
	}
}