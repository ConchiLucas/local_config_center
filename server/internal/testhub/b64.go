package testhub

import "encoding/base64"

func stdEncoding() *base64.Encoding { return base64.StdEncoding }
func rawEncoding() *base64.Encoding { return base64.RawStdEncoding }
