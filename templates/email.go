package templates

const SendAppointmentReminderEmailTemplate = `
<html>
	<head>
		<style>
			body { font-family: Arial, sans-serif; color: #333; line-height: 1.5; margin: 0; padding: 20px; }
			.order-details { margin: 20px 0; }
			.details-row { margin-bottom: 8px; }
			.label { font-weight: bold; display: inline; }
			.value { display: inline; margin-left: 5px; }
			.carton-table { width: 100%%; border-collapse: collapse; font-size: 14px; margin-top: 10px; }
			.carton-table th, .carton-table td { border: 1px solid #ddd; padding: 8px; text-align: left; }
			.carton-table th { background: #f5f5f5; font-weight: bold; }
			.location-section { margin: 20px 0; }
			.location-title { font-weight: bold; margin-bottom: 8px; }
			.location-row { margin-bottom: 6px; font-size: 14px; }
			.footer { margin-top: 30px; }
		</style>
	</head>
	<body>
		<p>Hello %s Team,</p>
		<p>This is a gentle reminder about the upcoming delivery for the following order. Kindly ensure all arrangements are made for an on-time and smooth delivery.</p>

		<div class="order-details">
			<div class="details-row">
				<span class="label">LR Number:</span>
				<span class="value">%s</span>
			</div>
			<div class="details-row">
				<span class="label">PO Number:</span>
				<span class="value">%s</span>
			</div>
			<div class="details-row">
				<span class="label">Master Waybills:</span>
				<span class="value">%s</span>
			</div>
			<div class="details-row">
				<span class="label">Child Waybills:</span>
				<span class="value">%s</span>
			</div>
			<div class="details-row">
				<span class="label">Delivery Schedule:</span>
				<span class="value">%s</span>
			</div>
			<div class="details-row">
				<span class="label">Total Cartons:</span>
				<span class="value">%d</span>
			</div>
			<div class="details-row">
				<span class="label">Total Weight:</span>
				<span class="value">%.2f KG</span>
			</div>
		</div>

		<div class="order-details">
			<div class="location-title">Carton Details</div>
			<table class="carton-table">
				<tr>
					<th>Carton</th>
					<th>Dimensions (L x B x H)</th>
					<th>Quantity</th>
					<th>Weight</th>
				</tr>
				%s
			</table>
		</div>

		<div class="location-section">
			<div class="location-title">Delivery Address</div>
			<div class="location-row">
				<span class="label">Name:</span>
				<span class="value">%s</span>
			</div>
			<div class="location-row">
				<span class="label">Address:</span>
				<span class="value">%s<br>%s, %s - %s</span>
			</div>
			<div class="location-row">
				<span class="label">Contact:</span>
				<span class="value">%s</span>
			</div>
			<div class="location-row">
				<span class="label">Email:</span>
				<span class="value">%s</span>
			</div>
		</div>

		<div class="location-section">
			<div class="location-title">Pickup Address</div>
			<div class="location-row">
				<span class="label">Name:</span>
				<span class="value">%s</span>
			</div>
			<div class="location-row">
				<span class="label">Address:</span>
				<span class="value">%s<br>%s, %s - %s</span>
			</div>
			<div class="location-row">
				<span class="label">Contact:</span>
				<span class="value">%s</span>
			</div>
			<div class="location-row">
				<span class="label">Email:</span>
				<span class="value">%s</span>
			</div>
		</div>

		<div class="footer">
			<p><strong>Best regards,</strong><br>Openleaf Team</p>
		</div>
	</body>
</html>
`

const SendAppointmentEmailTemplate = `
<html>
	<head>
		<style>
			body { font-family: Arial, sans-serif; color: #333; line-height: 1.5; margin: 0; padding: 20px; }
			.order-details { margin: 20px 0; }
			.details-row { margin-bottom: 8px; }
			.label { font-weight: bold; display: inline; }
			.value { display: inline; margin-left: 5px; }
			.carton-table { width: 100%%; border-collapse: collapse; font-size: 14px; margin-top: 10px; }
			.carton-table th, .carton-table td { border: 1px solid #ddd; padding: 8px; text-align: left; }
			.carton-table th { background: #f5f5f5; font-weight: bold; }
			.location-section { margin: 20px 0; }
			.location-title { font-weight: bold; margin-bottom: 8px; }
			.location-row { margin-bottom: 6px; font-size: 14px; }
			.footer { margin-top: 30px; }
		</style>
	</head>
	<body>
		<p>Hello %s Team,</p>
		<p>This is to inform you about the delivery schedule for the following order:</p>

		<div class="order-details">
			<div class="details-row">
				<span class="label">LR Number:</span>
				<span class="value">%s</span>
			</div>
			<div class="details-row">
				<span class="label">PO Number:</span>
				<span class="value">%s</span>
			</div>
			<div class="details-row">
				<span class="label">Master Waybills:</span>
				<span class="value">%s</span>
			</div>
			<div class="details-row">
				<span class="label">Child Waybills:</span>
				<span class="value">%s</span>
			</div>
			<div class="details-row">
				<span class="label">Delivery Schedule:</span>
				<span class="value">%s</span>
			</div>
			<div class="details-row">
				<span class="label">Total Cartons:</span>
				<span class="value">%d</span>
			</div>
			<div class="details-row">
				<span class="label">Total Weight:</span>
				<span class="value">%.2f KG</span>
			</div>
		</div>

		<div class="order-details">
			<div class="location-title">Carton Details</div>
			<table class="carton-table">
				<tr>
					<th>Carton</th>
					<th>Dimensions (L x B x H)</th>
					<th>Quantity</th>
					<th>Weight</th>
				</tr>
				%s
			</table>
		</div>

		<div class="location-section">
			<div class="location-title">Delivery Address</div>
			<div class="location-row">
				<span class="label">Name:</span>
				<span class="value">%s</span>
			</div>
			<div class="location-row">
				<span class="label">Address:</span>
				<span class="value">%s<br>%s, %s - %s</span>
			</div>
			<div class="location-row">
				<span class="label">Contact:</span>
				<span class="value">%s</span>
			</div>
			<div class="location-row">
				<span class="label">Email:</span>
				<span class="value">%s</span>
			</div>
		</div>

		<div class="location-section">
			<div class="location-title">Pickup Address</div>
			<div class="location-row">
				<span class="label">Name:</span>
				<span class="value">%s</span>
			</div>
			<div class="location-row">
				<span class="label">Address:</span>
				<span class="value">%s<br>%s, %s - %s</span>
			</div>
			<div class="location-row">
				<span class="label">Contact:</span>
				<span class="value">%s</span>
			</div>
			<div class="location-row">
				<span class="label">Email:</span>
				<span class="value">%s</span>
			</div>
		</div>

		<div class="footer">
			<p><strong>Best regards,</strong><br>Openleaf Team</p>
		</div>
	</body>
</html>
`

const SendCarrierBulkPickupEmailTemplate = `
<html>
    <head>
        <style>
            body { font-family: Arial, sans-serif; color: #333; line-height: 1.5; margin: 0; padding: 20px; }
            .order-details { margin: 20px 0; }
            .details-row { margin-bottom: 8px; }
            .label { font-weight: bold; display: inline; }
            .value { display: inline; margin-left: 5px; }
            .data-table { width: 100%%; border-collapse: collapse; font-size: 14px; margin-top: 10px; }
            .data-table th, .data-table td { 
                border: 1px solid #ddd; 
                padding: 8px; 
                text-align: left; 
                vertical-align: top; 
                white-space: nowrap;
				text-align: center;
            }
            .data-table th { background: #f5f5f5; font-weight: bold; }
            .sub-label { font-weight: bold; }
            .location-section { margin: 20px 0; }
            .location-title { font-weight: bold; margin-bottom: 8px; }
            .location-row { margin-bottom: 6px; font-size: 14px; }
            .footer { margin-top: 30px; }
        </style>
    </head>
    <body>
        <p>Hello Team,</p>
        <p>This is to inform you about the Pick Up schedule for the following order(s):</p>

        <div class="order-details">
            <div class="details-row">
                <span class="label">Total Cartons:</span>
                <span class="value">%d</span>
            </div>
            <div class="details-row">
                <span class="label">Total Weight:</span>
                <span class="value">%.2f KG</span>
            </div>
            <div class="details-row">
                <span class="label">Total LRs:</span>
                <span class="value">%d</span>
            </div>
        </div>

        <div class="order-details">
            <div class="location-title">Shipment Details</div>
            <table class="data-table">
                <tr>
                    <th rowspan="2" style="text-align:center;vertical-align:middle;">Sr No</th>
                    <th colspan="6" style="text-align:center;vertical-align:middle;">PO Details</th>
                    <th rowspan="2" style="text-align:center;vertical-align:middle;">LR Number</th>
                    <th rowspan="2" style="text-align:center;vertical-align:middle;">Appointment Date</th>
                    <th colspan="3" style="text-align:center;vertical-align:middle;">Carton Details</th>
                    <th colspan="3" style="text-align:center;vertical-align:middle;">Invoice Details</th>
                </tr>
                <tr>
                    <th>Channel</th>
                    <th>Number</th>
                    <th>Date</th>
                    <th>City</th>
                    <th>Pincode</th>
                    <th>Amount</th>
                    <th>Quantity</th>
                    <th>Weight</th>
                    <th>Dimensions (L x B x H)</th>
                    <th>Number</th>
                    <th>Quantity</th>
                    <th>Amount</th>
                </tr>
                %s
            </table>
        </div>

        <div class="footer">
            <p><strong>Best regards,</strong><br>Openleaf Team</p>
        </div>
    </body>
</html>
`