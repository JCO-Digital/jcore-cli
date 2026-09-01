package update

// PublicKey is the Ed25519 public key used to verify the authenticity of
// jcore release binaries. It is paired with a private key that is only
// available in CI (stored as the MINISIGN_PRIVATE_KEY GitHub secret) and
// used by cmd/sign-binaries to sign each release asset.
//
// This is the same simplified "minisig" scheme used by jman: a raw
// base64-encoded Ed25519 signature over the exact file bytes, stored
// alongside the asset as "<asset>.minisig". It is not the on-disk format
// produced by the real minisign tool.
const PublicKey = "xP/r9Z5N72DvY+RDL2DPM65mrBJtbskJgoY12ybq9HA="
