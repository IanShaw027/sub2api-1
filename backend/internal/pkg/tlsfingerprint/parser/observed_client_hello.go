package parser

// ObservedClientHello stores replay-relevant observed fields from a raw ClientHello.
type ObservedClientHello struct {
	RawClientHello                 []byte
	LegacyVersion                  uint16
	RecordVersion                  uint16
	ClientHelloVersion             uint16
	SniPresent                     bool
	GreaseValues                   []uint16
	CipherSuites                   []uint16
	Curves                         []uint16
	PointFormats                   []uint16
	SignatureAlgorithms            []uint16
	SignatureAlgorithmsCert        []uint16
	ExtensionsOrder                []uint16
	ExtensionMetadata              map[uint16][]byte
	ALPNProtocols                  []string
	SupportedVersions              []uint16
	KeyShareGroups                 []uint16
	PSKModes                       []uint16
	CompressCertAlgos              []uint16
	DelegatedCredentialsAlgorithms []uint16
	ApplicationSettingsProtocols   []string
	EnableGREASE                   bool
}

// DerivedFingerprint stores analysis metadata derived from the observed hello.
type DerivedFingerprint struct {
	ReplayHash        string
	ReplayHashVersion string
	JA3Raw            string
	JA3Hash           string
	JA4               string
	ALPNFingerprint   string
	Http2Fingerprint  string
	ParseVersion      string
}
