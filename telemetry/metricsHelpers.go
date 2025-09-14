package telemetry

// IncTransactionsProcessed increments the business success counter.
func IncTransactionsProcessed() {
	transactionsProcessedTotal.Inc()
}

// Increments the business failure counter
// Reasons: "validation", "db", "schema", "kafka".
func IncTransactionsFailed(reason string) {
	transactionsFailedTotal.WithLabelValues(reason).Inc()
}

// Sets the current queue size gauge.
func SetWorkerQueueCurrent(n int) {
	workerQueueCurrent.Set(float64(n))
}

// Increments both the created counter and the current total gauge.
func IncUsersCreated() {
	usersCreatedTotal.Inc()
	usersTotalCurrent.Inc()
}

// Increments the failed-create counter with a bounded reason.
func IncUsersCreateFailed(reason string) {
	if reason == "" {
		reason = "unknown"
	}
	usersCreateFailedTotal.WithLabelValues(reason).Inc()
}

// Increments the GET counter labeled by whether the user was found.
func IncUsersGet(found bool) {
	lbl := "false"
	if found {
		lbl = "true"
	}
	usersGetTotal.WithLabelValues(lbl).Inc()
}

// Sets Users total.
func SetUsersTotal(n int) {
	usersTotalCurrent.Set(float64(n))
}
