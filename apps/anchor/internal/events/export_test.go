package events

func ValidateEndpointURLForTest(raw string, production bool) error {
	return validateEndpointURL(raw, production)
}

func DeliveredDataForTest(data Data) Data {
	return deliveredData(data)
}
