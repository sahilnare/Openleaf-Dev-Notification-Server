package cron

import (
	"Openleaf-Dev-B2B-Appointment-Server/db"
	"database/sql"
	"log"
	"time"
)

func RunCronJob() {
	log.Println("Starting cron job execution")
	dbConn := db.GetDB()
	if dbConn == nil {
		log.Println("DB not initialized")
		return
	}

	// 1. Process regular logistics requests (is_processed = 'N', phase != 2, is_reminder != true)
	pendingAppointments, err := getPendingAppointments(dbConn)
	if err != nil {
		log.Printf("Error fetching pending appointments: %v", err)
	} else {
		log.Printf("Found %d pending logistics requests", len(pendingAppointments))
		for _, appt := range pendingAppointments {
			success := sendLogisticsCoordinationEmailStub(appt)
			markAppointmentProcessed(dbConn, appt.ID, success)
			if success {
				log.Printf("Logistics request %d sent successfully", appt.ID)
			} else {
				log.Printf("Logistics request %d failed to send", appt.ID)
			}
			time.Sleep(1 * time.Second)
		}
	}

	// 2. Process phase 2 carrier emails (is_processed = 'N', phase = 2, carrier_email_id not null)
	phase2Appointments, err := getPhase2Appointments(dbConn)
	if err != nil {
		log.Printf("Error fetching phase 2 appointments: %v", err)
	} else {
		log.Printf("Found %d phase 2 carrier appointments", len(phase2Appointments))
		for _, appt := range phase2Appointments {
			success := sendCarrierEmailStub(appt)
			markPhase2AppointmentProcessed(dbConn, appt.ID, success)
			if success {
				log.Printf("Phase 2 carrier email %d sent successfully", appt.ID)
			} else {
				log.Printf("Phase 2 carrier email %d failed to send", appt.ID)
			}
			time.Sleep(1 * time.Second)
		}
	}

	// 3. Process reminder emails (is_reminder = true)
	reminderAppointments, err := getReminderAppointments(dbConn)
	if err != nil {
		log.Printf("Error fetching reminder appointments: %v", err)
	} else {
		log.Printf("Found %d reminder appointments", len(reminderAppointments))
		for _, appt := range reminderAppointments {
			success := sendReminderEmailStub(appt)
			markReminderAppointmentProcessed(dbConn, appt.ID, success)
			if success {
				log.Printf("Reminder email %d sent successfully", appt.ID)
			} else {
				log.Printf("Reminder email %d failed to send", appt.ID)
			}
			time.Sleep(1 * time.Second)
		}
	}

	log.Println("Cron job execution completed")
}

// --- DB Query Helpers ---

func getPendingAppointments(dbConn *sql.DB) ([]db.AppointmentScheduling, error) {
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	now := time.Now()
	rows, err := dbConn.Query(`
		SELECT id, user_id, schedule_datetime, is_processed, email_id, order_id, po_ids, cc_email_id, phase, carrier_email_id, carrier_cc_email_id, is_reminder
		FROM appointments_scheduling
		WHERE schedule_datetime BETWEEN $1 AND $2
		AND is_processed = 'N'
		AND (phase IS NULL OR phase != 2)
		AND (is_reminder IS NULL OR is_reminder != TRUE)
	`, oneHourAgo, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var appts []db.AppointmentScheduling
	for rows.Next() {
		var a db.AppointmentScheduling
		var phase sql.NullInt64
		var carrierEmailID, carrierCcEmailID, orderID, poIDs, ccEmailID sql.NullString
		var isReminder sql.NullBool
		if err := rows.Scan(&a.ID, &a.UserID, &a.ScheduleDatetime, &a.IsProcessed, &a.EmailID, &orderID, &poIDs, &ccEmailID, &phase, &carrierEmailID, &carrierCcEmailID, &isReminder); err != nil {
			return nil, err
		}
		if orderID.Valid {
			tmp := orderID.String
			a.OrderID = &tmp
		}
		if poIDs.Valid {
			tmp := poIDs.String
			a.PoIDs = &tmp
		}
		if ccEmailID.Valid {
			tmp := ccEmailID.String
			a.CcEmailID = &tmp
		}
		if phase.Valid {
			tmp := int(phase.Int64)
			a.Phase = &tmp
		}
		if carrierEmailID.Valid {
			tmp := carrierEmailID.String
			a.CarrierEmailID = &tmp
		}
		if carrierCcEmailID.Valid {
			tmp := carrierCcEmailID.String
			a.CarrierCcEmailID = &tmp
		}
		if isReminder.Valid {
			tmp := isReminder.Bool
			a.IsReminder = &tmp
		}
		appts = append(appts, a)
	}
	return appts, nil
}

func getPhase2Appointments(dbConn *sql.DB) ([]db.AppointmentScheduling, error) {
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	now := time.Now()
	rows, err := dbConn.Query(`
		SELECT id, user_id, schedule_datetime, is_processed, email_id, order_id, po_ids, cc_email_id, phase, carrier_email_id, carrier_cc_email_id, is_reminder
		FROM appointments_scheduling
		WHERE schedule_datetime BETWEEN $1 AND $2
		AND is_processed = 'N'
		AND phase = 2
		AND carrier_email_id IS NOT NULL
		AND carrier_cc_email_id IS NOT NULL
	`, oneHourAgo, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var appts []db.AppointmentScheduling
	for rows.Next() {
		var a db.AppointmentScheduling
		var phase sql.NullInt64
		var carrierEmailID, carrierCcEmailID, orderID, poIDs, ccEmailID sql.NullString
		var isReminder sql.NullBool
		if err := rows.Scan(&a.ID, &a.UserID, &a.ScheduleDatetime, &a.IsProcessed, &a.EmailID, &orderID, &poIDs, &ccEmailID, &phase, &carrierEmailID, &carrierCcEmailID, &isReminder); err != nil {
			return nil, err
		}
		if orderID.Valid {
			tmp := orderID.String
			a.OrderID = &tmp
		}
		if poIDs.Valid {
			tmp := poIDs.String
			a.PoIDs = &tmp
		}
		if ccEmailID.Valid {
			tmp := ccEmailID.String
			a.CcEmailID = &tmp
		}
		if phase.Valid {
			tmp := int(phase.Int64)
			a.Phase = &tmp
		}
		if carrierEmailID.Valid {
			tmp := carrierEmailID.String
			a.CarrierEmailID = &tmp
		}
		if carrierCcEmailID.Valid {
			tmp := carrierCcEmailID.String
			a.CarrierCcEmailID = &tmp
		}
		if isReminder.Valid {
			tmp := isReminder.Bool
			a.IsReminder = &tmp
		}
		appts = append(appts, a)
	}
	return appts, nil
}

func getReminderAppointments(dbConn *sql.DB) ([]db.AppointmentScheduling, error) {
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	now := time.Now()
	rows, err := dbConn.Query(`
		SELECT id, user_id, schedule_datetime, is_processed, email_id, order_id, po_ids, cc_email_id, phase, carrier_email_id, carrier_cc_email_id, is_reminder
		FROM appointments_scheduling
		WHERE schedule_datetime BETWEEN $1 AND $2
		AND is_processed = 'N'
		AND is_reminder = TRUE
		AND carrier_email_id IS NOT NULL
		AND carrier_cc_email_id IS NOT NULL
	`, oneHourAgo, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var appts []db.AppointmentScheduling
	for rows.Next() {
		var a db.AppointmentScheduling
		var phase sql.NullInt64
		var carrierEmailID, carrierCcEmailID, orderID, poIDs, ccEmailID sql.NullString
		var isReminder sql.NullBool
		if err := rows.Scan(&a.ID, &a.UserID, &a.ScheduleDatetime, &a.IsProcessed, &a.EmailID, &orderID, &poIDs, &ccEmailID, &phase, &carrierEmailID, &carrierCcEmailID, &isReminder); err != nil {
			return nil, err
		}
		if orderID.Valid {
			tmp := orderID.String
			a.OrderID = &tmp
		}
		if poIDs.Valid {
			tmp := poIDs.String
			a.PoIDs = &tmp
		}
		if ccEmailID.Valid {
			tmp := ccEmailID.String
			a.CcEmailID = &tmp
		}
		if phase.Valid {
			tmp := int(phase.Int64)
			a.Phase = &tmp
		}
		if carrierEmailID.Valid {
			tmp := carrierEmailID.String
			a.CarrierEmailID = &tmp
		}
		if carrierCcEmailID.Valid {
			tmp := carrierCcEmailID.String
			a.CarrierCcEmailID = &tmp
		}
		if isReminder.Valid {
			tmp := isReminder.Bool
			a.IsReminder = &tmp
		}
		appts = append(appts, a)
	}
	return appts, nil
}

// --- Mark as processed helpers ---

func markAppointmentProcessed(dbConn *sql.DB, id int, success bool) {
	status := "N"
	if success {
		status = "Y"
	}
	_, err := dbConn.Exec(`UPDATE appointments_scheduling SET is_processed = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, status, id)
	if err != nil {
		log.Printf("Error marking appointment %d as processed: %v", id, err)
	}
}

func markPhase2AppointmentProcessed(dbConn *sql.DB, id int, success bool) {
	_, err := dbConn.Exec(`UPDATE appointments_scheduling SET is_processed = 'Y', updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	if err != nil {
		log.Printf("Error marking phase2 appointment %d as processed: %v", id, err)
	}
}

func markReminderAppointmentProcessed(dbConn *sql.DB, id int, success bool) {
	status := "N"
	if success {
		status = "Y"
	}
	_, err := dbConn.Exec(`UPDATE appointments_scheduling SET is_processed = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, status, id)
	if err != nil {
		log.Printf("Error marking reminder appointment %d as processed: %v", id, err)
	}
}

// --- Email sending stubs ---

func sendLogisticsCoordinationEmailStub(appt db.AppointmentScheduling) bool {
	log.Printf("[STUB] Would send logistics coordination email to %s for POs: %v", appt.EmailID, appt.PoIDs)
	return true
}

func sendCarrierEmailStub(appt db.AppointmentScheduling) bool {
	log.Printf("[STUB] Would send carrier email to %v for order: %v", appt.CarrierEmailID, appt.OrderID)
	return true
}

func sendReminderEmailStub(appt db.AppointmentScheduling) bool {
	log.Printf("[STUB] Would send reminder email to %v for order: %v", appt.CarrierEmailID, appt.OrderID)
	return true
} 