package specs

import _ "embed"

// BundleSpecification is the bundle file specification implemented by this
// version of par2cron, embedded from bundle_specification.txt.
//
//go:embed bundle_specification.txt
var BundleSpecification string
