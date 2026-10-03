package configs

import _ "embed"

// ExampleConfiguration is the full configuration example as supported by this
// version of par2cron, embedded from par2cron.yaml.
//
//go:embed par2cron.yaml
var ExampleConfiguration string
