package status

type Battery struct {
	Percent  int  `json:"percent"`
	Charging bool `json:"charging"`
}

type Wifi struct {
	Connected bool `json:"connected"`
	Strength  int  `json:"strength"`
}

type Status struct {
	Battery Battery `json:"battery"`
	Wifi    Wifi    `json:"wifi"`
}
