package snapshot

// Compatibility notes for Snapshot Schema v1.
//
// Unknown schemaVersion / artifactVersion → reject.
// Unknown requiredFeatures entries → reject.
// Unknown additive JSON fields in an otherwise supported v1 document → accept
// and ignore (Decode does not use DisallowUnknownFields).
// Duplicate object keys → reject.
// Trailing non-whitespace JSON tokens → reject.
//
// The embedded fingerprint attests the structural artifact only. It is not a
// checksum of the entire snapshot JSON document or of Capture Semantics.
const CompatibilityPolicy = "snapshot-schema-v1-additive"
