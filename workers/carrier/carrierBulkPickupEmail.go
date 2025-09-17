package carrierWorker

import (
	"Notification-Server/helpers"
	"Notification-Server/models"
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
)

func SendCarrierBulkPickupEmail(ctx context.Context, task *asynq.Task) error {
	defer func() {
		if r := recover(); r != nil {
			helpers.LogException("[worker] carrier bulk pickup email worker panic recovered", map[string]interface{}{
				"panic":     r,
				"task_type": task.Type(),
				"task_data": string(task.Payload()),
			})
		}
	}()

	helpers.LogInfo("[worker] carrier bulk pickup email worker started", map[string]interface{}{
		"task_type":      task.Type(),
		"task_data":      string(task.Payload()),
		"payload_length": len(task.Payload()),
	})

	if task == nil {
		helpers.LogInfo("[worker] carrier bulk pickup email task is nil", map[string]interface{}{
			"task_data": string(task.Payload()),
			"task_type": task.Type(),
		})
		return nil
	}

	payload := task.Payload()

	helpers.LogInfo("[worker] carrier bulk pickup email payload raw", map[string]interface{}{
		"task_type": task.Type(),
		"task_data": string(task.Payload()),
		"payload":   string(payload),
	})

	var data models.CarrierBulkPickupEmailWorkerData
	err := json.Unmarshal(payload, &data)
	if err != nil {
		helpers.LogException("[worker] carrier bulk pickup email failed to unmarshal payload", map[string]interface{}{
			"error":     err.Error(),
			"task_type": task.Type(),
			"payload":   string(payload),
		})
		return err
	}

	helpers.LogInfo("[worker] carrier bulk pickup email payload unmarshaled", map[string]interface{}{
		"task_type": task.Type(),
		"data":      data,
	})

	// todo fetch data

	// todo send email
	

	helpers.LogInfo("[worker] carrier bulk pickup email worker completed successfully", map[string]interface{}{
		"task_type": task.Type(),
		"data":      data,
	})

	return nil
}
