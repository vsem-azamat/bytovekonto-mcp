package fakeapi

// DocumentContent is the body of every downloaded document.
const DocumentContent = "%PDF-1.4 example house rules\n"

// QRPNG is the base64 of a 1x1 PNG, standing in for a payment QR code.
const QRPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

// MeID is the subject id of the fake's user.
const MeID = 1

// fixtures are the GET answers by path, shaped like the real API's, with made-up data: one
// tenant of flat A101 in "Example House", two own tickets and one public ticket of the building.
var fixtures = map[string]string{
	"/home-account": `[{"id":100,"name":"A101_Example","subType":"TENANCY","projectName":"Example House","activeFrom":"2026-01-01T00:00:00Z","activeTo":"2026-12-31T00:00:00Z","activeFromDate":"2026-01-01","activeToDate":"2026-12-31","active":true}]`,

	"/home-account/detail": `{"id":100,"name":"A101_Example","subType":"TENANCY","active":true,"activeFrom":"2026-01-01T00:00:00Z","activeTo":"2026-12-31T00:00:00Z",
		"subjects":[{"id":1,"name":"Jana Example","address":null,"type":"PERSON","relationshipName":"Jana Example","relationshipType":"TENANT","subType":"PRIMARY","identifier":null,"validFrom":"2026-01-01","validTo":"2026-12-31","responsible":true,"note":"","mailingAddress":null,"email":null,"phone":null,"bankAccount":null}],
		"objects":[{"id":701,"name":"A101","type":"FLAT","address":"Example Street 1, Praha","dateFrom":null,"dateTo":null,"objectData":[]}],
		"prescriptionPaymentData":{"status":"ACTIVE","stable":true,"issueDay":1,"dueDays":14,"vs":"1234567","ss":"12026","supplierName":"Example Owner s.r.o.","bankAccountNumber":"123456789","bankCode":"0800","totalAmount":25000.0,"currency":"CZK"}}`,

	"/project/info": `[{"relatedToUnit":"IDENTITY","relatedToUnitAs":"IDENTITY","id":10,"type":"COMPANY","name":"Example Manager s.r.o.","address":"Example Square 2, Praha","identifier":"11111111","dic":"CZ11111111",
		"bankConnections":[{"id":1,"paymentName":"main","paymentDataType":"BANK","accountIdentifier":"987654321","bankCode":"0800","creationOrderNr":1,"iban":null,"currency":"CZK","preferred":true}],
		"contacts":[{"id":20,"name":"Emergency line","type":"PERSON","relationshipName":"Emergency line","relationshipType":"CONTACT","subType":"EMERGENCY_SERVICE","identifier":"x","validFrom":"2025-01-01","validTo":null,"responsible":false,"note":"","email":null,"phone":{"id":1,"value":"600 000 000"}}],
		"suppliers":[],"contactInfo":[]},
		{"relatedToUnit":"PROJECT","relatedToUnitAs":"PRIMARY","id":11,"type":"COMPANY","name":"Example Owner s.r.o.","address":"Example Square 2, Praha","identifier":"22222222","dic":"CZ22222222","bankConnections":[],"contacts":[],"suppliers":[],"contactInfo":[]}]`,

	"/project/contacts": `[{"id":20,"name":"Emergency line","type":"PERSON","relationshipName":"Emergency line","relationshipType":"CONTACT","subType":"EMERGENCY_SERVICE","identifier":"x","validFrom":"2025-01-01","validTo":null,"responsible":false,"note":"","email":null,"phone":{"id":1,"value":"600 000 000"}},
		{"id":21,"name":"Tom Technician","type":"PERSON","relationshipName":"Tom Technician","relationshipType":"CONTACT","subType":"TECHNICIAN","identifier":"y","validFrom":"2025-01-01","validTo":null,"responsible":false,"note":"","email":"tom@example.com","phone":null}]`,

	"/project/infoboxes": `[{"id":-500,"identityId":10,"projectId":500,"activeFrom":null,"activeTo":null,"priority":0,"layout":"image_left","backgroundColor":"#000","textColor":"#fff","imageUrl":"https://example.com/logo.png","linkUrl":null,"htmlContent":"<p><b>Water off</b> on Monday&nbsp;8&ndash;12</p><p>Managed by Example Manager</p>"}]`,

	"/project/faq": `[{"question":"Where is the bike room?","answer":"In the basement, level -1."}]`,

	"/tickets/list-with-last-comment": `[
		{"id":3001,"createdAt":"2026-03-01T10:00:00Z","modifiedAt":"2026-03-02T10:00:00Z","closedAt":null,"subject":"Leaking tap","description":"The kitchen tap leaks.","ticketState":"IN_PROGRESS","possibleTicketStates":[],"ticketPriority":"NORMAL","dueDate":null,"ticketCategoryId":2,"ticketCategory":"Fault","ticketAreaId":28,"ticketArea":"Other","projectName":"Example House","tags":["USER_IS_CREATOR","STATE_OPEN"],"ticketMembersContactInfo":[{"anonymized":false,"roleType":"SOLVER","subjectId":21,"label":"Tom Technician","accountLabel":null,"phone":null,"email":null}],"relations":[{"entityType":"PROJECT","entityId":"500","label":"Example House"},{"entityType":"OBJECT","entityId":"701","label":"A101"}],"privateTicket":false,"noComments":false,"ticketMemberCount":2,"pinned":false,"lastCommentText":"We will come on Friday.","lastCommentAuthorSubjectId":21,"lastCommentAuthor":"Tom Technician","lastCommentCreatedAt":"2026-03-02T10:00:00Z","allCommentAuthorSubjectIds":[21],"allCommentAuthorRelationshipIds":[]},
		{"id":3000,"createdAt":"2026-02-01T10:00:00Z","modifiedAt":"2026-02-03T10:00:00Z","closedAt":"2026-02-03T10:00:00Z","subject":"Bike room key","description":"Where do I get a key?","ticketState":"SOLVED","possibleTicketStates":[],"ticketPriority":"NORMAL","dueDate":null,"ticketCategoryId":4,"ticketCategory":"Question","ticketAreaId":28,"ticketArea":"Other","projectName":"Example House","tags":["USER_IS_CREATOR","STATE_CLOSED"],"ticketMembersContactInfo":[],"relations":[{"entityType":"PROJECT","entityId":"500","label":"Example House"},{"entityType":"OBJECT","entityId":"701","label":"A101"}],"privateTicket":false,"noComments":false,"ticketMemberCount":2,"pinned":false,"lastCommentText":null,"lastCommentAuthorSubjectId":null,"lastCommentAuthor":null,"lastCommentCreatedAt":null,"allCommentAuthorSubjectIds":[],"allCommentAuthorRelationshipIds":[]},
		{"id":2000,"createdAt":"2026-01-15T10:00:00Z","modifiedAt":"2026-01-15T10:00:00Z","closedAt":null,"subject":"Lift out of order","description":"The lift in entrance B stops.","ticketState":"NEW","possibleTicketStates":[],"ticketPriority":"NORMAL","dueDate":null,"ticketCategoryId":2,"ticketCategory":"Fault","ticketAreaId":28,"ticketArea":"Other","projectName":"Example House","tags":["INVOLVES_COMMON_AREA","STATE_OPEN"],"ticketMembersContactInfo":[],"relations":[{"entityType":"PROJECT","entityId":"500","label":"Example House"},{"entityType":"OBJECT","entityId":"500","label":"Example House"}],"privateTicket":false,"noComments":true,"ticketMemberCount":3,"pinned":false,"lastCommentText":null,"lastCommentAuthorSubjectId":null,"lastCommentAuthor":null,"lastCommentCreatedAt":null,"allCommentAuthorSubjectIds":[],"allCommentAuthorRelationshipIds":[]}]`,

	"/tickets/3001": `{"id":3001,"createdAt":"2026-03-01T10:00:00Z","modifiedAt":"2026-03-02T10:00:00Z","closedAt":null,"subject":"Leaking tap","description":"The kitchen tap leaks.","ticketState":"IN_PROGRESS","possibleTicketStates":[],"ticketPriority":"NORMAL","dueDate":null,"ticketCategoryId":2,"ticketCategory":"Fault","ticketAreaId":28,"ticketArea":"Other","projectName":"Example House","tags":["USER_IS_CREATOR","STATE_OPEN"],"ticketMembersContactInfo":[{"anonymized":false,"roleType":"SOLVER","subjectId":21,"label":"Tom Technician","accountLabel":null,"phone":null,"email":null}],"relations":[{"entityType":"PROJECT","entityId":"500","label":"Example House"},{"entityType":"OBJECT","entityId":"701","label":"A101"}],"privateTicket":false,"noComments":false,"ticketMemberCount":2,"pinned":false,"relatedEntities":[],"actionTokens":{},"actions":[],"attachments":[],"ignoreMemberCountWarning":false,"checklist":[],"subtickets":[],"pinnedForAll":false,"customReminderSchema":null}`,

	"/tickets/3001/lifecycle": `{"comments":[
		{"id":2,"createdAt":"2026-03-02T10:00:00Z","creatorSubjectId":21,"creatorSubjectName":"Tom Technician","text":"We will come on Friday.","attachmentIds":[],"modifiedAt":"2026-03-02T10:00:00Z","messageType":null,"deletedAt":null,"deletedBySubjectId":null,"deletedByName":"","allowEdit":false},
		{"id":1,"createdAt":"2026-03-01T12:00:00Z","creatorSubjectId":1,"creatorSubjectName":"Jana Example","text":"It drips all night.","attachmentIds":[],"modifiedAt":"2026-03-01T12:00:00Z","messageType":null,"deletedAt":null,"deletedBySubjectId":null,"deletedByName":"","allowEdit":true},
		{"id":3,"createdAt":"2026-03-01T13:00:00Z","creatorSubjectId":1,"creatorSubjectName":"Jana Example","text":"deleted","attachmentIds":[],"modifiedAt":"2026-03-01T13:00:00Z","messageType":null,"deletedAt":"2026-03-01T13:05:00Z","deletedBySubjectId":1,"deletedByName":"Jana Example","allowEdit":false}],
		"history":[{"metadata":{"id":1,"eventType":"STATE_CHANGED","representationType":"x","objectType":"TICKET","objectTypeMapped":"TICKET","objectId":3001,"objectVersion":2,"otherObjectType":null,"otherObjectTypeMapped":null,"otherObjectId":null,"context":"x","subjectName":"Tom Technician","created":"2026-03-02T09:00:00Z","received":"2026-03-02T09:00:00Z","effectiveDateTime":"2026-03-02T09:00:00Z"},"changes":[{"field":"ticketState","oldValue":"NEW","newValue":"IN_PROGRESS"}]}],
		"attachmentMetadata":null}`,

	"/tickets/2000": `{"id":2000,"createdAt":"2026-01-15T10:00:00Z","modifiedAt":"2026-01-15T10:00:00Z","closedAt":null,"subject":"Lift out of order","description":"The lift in entrance B stops.","ticketState":"NEW","possibleTicketStates":[],"ticketPriority":"NORMAL","dueDate":null,"ticketCategoryId":2,"ticketCategory":"Fault","ticketAreaId":28,"ticketArea":"Other","projectName":"Example House","tags":["INVOLVES_COMMON_AREA","STATE_OPEN"],"ticketMembersContactInfo":[],"relations":[],"privateTicket":false,"noComments":true,"ticketMemberCount":3,"pinned":false,"relatedEntities":[],"actionTokens":{},"actions":[],"attachments":[],"ignoreMemberCountWarning":false,"checklist":[],"subtickets":[],"pinnedForAll":false,"customReminderSchema":null}`,

	"/tickets/2000/lifecycle": `{"comments":[],"history":[],"attachmentMetadata":null}`,

	"/tickets/options": `{"categories":[{"id":1,"code":"ACCIDENT","type":"CATEGORY","name":"Accident"},{"id":2,"code":"FAULT","type":"CATEGORY","name":"Fault"},{"id":4,"code":"QUESTION","type":"CATEGORY","name":"Question"},{"id":9,"code":"ANNOUNCEMENT","type":"CATEGORY","name":"Announcement"}],
		"areas":[{"id":20,"code":"EQUIPMENT","type":"AREA","name":"Equipment"},{"id":28,"code":"AREA_OTHER","type":"AREA","name":"Other"}],
		"objects":[{"id":500,"name":"Example House","type":"BUILDING","own":false},{"id":701,"name":"A101","type":"FLAT","own":true},{"id":702,"name":"C01","type":"BASEMENT","own":false},{"id":703,"name":"P1","type":"PARKING","own":false},{"id":704,"name":"P1","type":"PARKING","own":false}],"items":[]}`,

	"/payments": `[{"id":7001,"totalAmount":25000.00,"totalAmountPaid":0,"remainingUnpaidAmount":25000.00,"amountPaidDirectly":0.00,"amountPaidLinked":0.00,"paymentStatus":"UNPAID","documentCurrency":"CZK","documentDefinitionName":"Prescription","documentDefinitionType":"PRESCRIPTION","sequentialNumber":"ZP1/2026","formalCreatedDate":"2026-03-01","formalPaymentDate":null,"formalPaymentDueDate":"2026-03-15","paymentPeriodType":"MONTH","prescriptionName":"2026-03","settlementName":null,"supplierName":"Example Owner s.r.o.","supplierICO":"22222222","customerName":"Jana Example","homeAccountId":100,"homeAccountName":"A101_Example","paymentAvailable":true}]`,

	"/payments/7001": `{"id":7001,"sequentialNumber":"ZP1/2026","documentDefinitionName":"Prescription","documentDefinitionType":"PRESCRIPTION","paymentStatus":"UNPAID","documentStatus":"APPROVED","paymentType":"BANK","totalAmount":25000.00,"totalAmountPaid":0,"remainingUnpaidAmount":25000.00,"documentCurrency":"CZK","bankPaymentVS":"1234567","bankPaymentSS":"32026","bankPaymentKS":null,"formalCreatedDate":"2026-03-01","formalPaymentDueDate":"2026-03-15","formalPaymentDate":null,"globalPeriodFrom":null,"globalPeriodTo":null,"notePublic":"2026-03-01 - 2026-03-31","prescriptionName":"2026-03","settlementName":null,
		"items":[{"description":"A101 - Rent","deliveryName":null,"amount":24000.0,"sumWithVat":24000.0,"itemCount":1,"itemUnit":"PIECE","vatRate":0,"periodValidFrom":null,"periodValidTo":null},{"description":"A101 - Heat","deliveryName":null,"amount":1000.0,"sumWithVat":1000.0,"itemCount":1,"itemUnit":"PIECE","vatRate":0,"periodValidFrom":null,"periodValidTo":null}]}`,

	"/payments/overview": `{"overview":{"pastDue":25000.0,"oldestPastDueDate":"2026-03-15","due":0,"nearestDueDate":null,"paid":0,"overpaid":0,"planned":0,"unpaidFromPreviousPeriods":0,"youngestTransactionDocumentDate":null},"monthsData":[{"month":"MARCH","total":25000,"paid":0,"pastDue":25000,"oldestPastDueDate":null,"due":0,"nearestDueDate":null,"actualDocumentCount":1,"plannedDocumentCount":0}]}`,

	"/costs": `{"currency":"CZK","currencyLocalized":"Kč","periods":[{"from":"2026-01-01T00:00:00+01:00","to":"2026-02-01T00:00:00+01:00","periodSchedule":"MONTH","label":"Jan"},{"from":"2026-02-01T00:00:00+01:00","to":"2026-03-01T00:00:00+01:00","periodSchedule":"MONTH","label":"Feb"}],
		"deliveries":[{"deliveryId":1,"objectId":701,"label":"Rent","category":"RENT","categoryDefinition":"SERVICE","complete":true,"totalCost":48000.0,"totalCostType":"PRESCRIPTION","prescriptionCost":48000.0,"consumption":0,"consumptionUnit":null,"periodCost":[24000,24000],"alerts":[],"periodConsumption":[0,0],"periodPrescription":[24000,24000],"previousCost":null,"rules":[],"noResults":null,"metersConsumption":[]},
		{"deliveryId":2,"objectId":701,"label":"Heat ","category":"HEAT","categoryDefinition":"CONSUMPTION","complete":false,"totalCost":2000.5,"totalCostType":"SETTLEMENT","prescriptionCost":2000.0,"consumption":300.25,"consumptionUnit":"kWh","periodCost":[1000.25,1000.25],"alerts":[],"periodConsumption":[150.5,149.75],"periodPrescription":[1000,1000],"previousCost":null,"rules":[],"noResults":null,"metersConsumption":[]}],
		"periodType":"UNSETTLED","dataAvailableTo":"2026-03-01T10:00:00+01:00","periodLengthDescription":"year to date"}`,

	"/costs/rent": `{"currency":"CZK","currencyLocalized":"Kč","current":{"from":"2026-01-01","to":null,"value":24000.0},"historic":[]}`,

	"/metering/": `[{"id":801,"type":"Electricity meter","category":"Meters","categoryCode":"METERS","typeCode":"ELECTRICITY_METER","modelName":"0001","name":"ELM_0001","objectNames":["A101"],"activeFrom":"2025-01-01","activeTo":null,"active":true,"subjects":[],"tickets":[],"qrCodes":[],"attachments":[],"srNumber":"0001","model":null,"purchaseDate":null,"numberOfPieces":null,"attributes":{},"lifecycle":{"id":1},"pricePurchase":null,"pricePurchaseVat":null}]`,

	"/metering/801": `{"id":801,"type":"Electricity meter","category":"Meters","categoryCode":"METERS","typeCode":"ELECTRICITY_METER","modelName":"0001","name":"ELM_0001","objectNames":["A101"],"activeFrom":"2025-01-01","activeTo":null,"active":true,
		"lastValues":{"P+ T2":{"unit":"kWh","channel":"P+ T2","lastValue":null,"lastValueTime":null,"lastValueOriginalTime":null},"P+ T1":{"unit":"kWh","channel":"P+ T1","lastValue":1234.5,"lastValueTime":"2026-03-01T10:00:00Z","lastValueOriginalTime":null}},
		"lastValueReceived":"2026-03-01T10:00:00Z","receptionStatus":"OK"}`,

	"/metering/801/chartData": `{"labels":["Jan","Feb"],"startTimeWithOffset":"2026-01-01T00:00:00+01:00","endTimeWithOffset":"2026-03-01T00:00:00+01:00","startTime":"x","endTime":"y",
		"dataByChannel":{"P+ T1":{"totalConsumption":200.0,"consumptions":[120.0,80.0],"startValue":1034.5,"endValue":1234.5,"unit":"kWh","partialData":[],"measuredValues":[]}},"coefficients":{}}`,

	"/documents/myProject": `[{"id":"doc-1","name":"House rules","type":"FILE","fileId":"f1","fileMeta":{"id":"f1","filename":"../house rules.pdf","size":29,"contentType":"application/pdf","relatedEntities":[],"uploadedBySubjectId":10,"uploadedBySubjectName":"Example Manager","createdAt":"2026-01-01T00:00:00Z"},"category":{"id":1,"code":"RULES","name":"Rules","nameUnaccented":"Rules"},"createdAt":"2026-01-01T00:00:00Z","modifiedAt":"2026-01-02T00:00:00Z"}]`,

	"/subject/me": `{"id":1,"login":"resident@example.com","anonymized":false,"data":[{"type":"PERSON","personData":{"firstName":"Jana","lastName":"Example"}}]}`,

	"/costs/compare": `{"currency":"CZK","currencyLocalized":"Kč","comparisons":[
		{"deliveryId":2,"objectId":701,"type":"PREVIOUS_PERIOD","available":true,"note":null,"from":"2026-01-01","to":"2026-01-31","totalUntil":null,"totalCost":40.5,"totalConsumption":0.25,"consumptionUnit":"kWh","periodCost":[40.5],"periodConsumption":[0.25]},
		{"deliveryId":2,"objectId":701,"type":"YEAR_AGO","available":false,"note":"Insufficient data for comparison","from":"2025-02-01","to":"2025-02-28","totalUntil":null,"totalCost":0,"totalConsumption":0,"consumptionUnit":null,"periodCost":[],"periodConsumption":[]},
		{"deliveryId":2,"objectId":701,"type":"PROJECT_AVG","available":true,"note":null,"from":"2026-02-01","to":"2026-02-28","totalUntil":null,"totalCost":820.25,"totalConsumption":5.2,"consumptionUnit":"kWh","periodCost":[820.25],"periodConsumption":[5.2]},
		{"deliveryId":3,"objectId":701,"type":"PREVIOUS_PERIOD","available":true,"note":null,"from":"2026-01-01","to":"2026-01-31","totalUntil":null,"totalCost":350,"totalConsumption":36.4,"consumptionUnit":"kWh","periodCost":[350],"periodConsumption":[36.4]}]}`,

	"/notifications/count": `2`,

	"/notifications/list": `{"notifications":[{"type":"TICKET_COMMENT","ticketId":3001}],"originalLastCheck":null}`,
}
