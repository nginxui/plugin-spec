// Package schema keeps schema/plugin.schema.json aligned with the Manifest
// message of manifest.proto. The proto is the source; the hand-written JSON
// Schema adds validation rules on top and its tests fail when the two drift.
package schema
