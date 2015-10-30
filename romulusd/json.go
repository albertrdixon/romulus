package main

import (
	"bytes"
	"fmt"
	"time"
)

type Duration time.Duration

func (d *Duration) UnmarshalJSON(p []byte) error {
	t, er := time.ParseDuration(string(bytes.Trim(p, `"`)))
	if er != nil {
		return er
	}
	(*d) = Duration(t)
	return nil
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, d.String())), nil
}

func (d *Duration) String() string {
	return (*time.Duration)(d).String()
}
