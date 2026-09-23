package v4

import (
	"github.com/toddwbucy/Aletharsis/internal/analyzers"
	"github.com/toddwbucy/Aletharsis/internal/identity"
	u "github.com/toddwbucy/Aletharsis/internal/unicoderef"
	"strings"
)

func (x *Index) validateDeclaredLimits() error {
	for _, p := range x.Packages {
		l := p.Limits
		if l.SourceBytes <= 0 || l.PartCount <= 0 || l.PartBytes <= 0 || l.AggregateBytes <= 0 ||
			l.MaxScopeTextUTF8Bytes <= 0 || l.MaxScopeScalarOrigins <= 0 || l.ReportInputBytes <= 0 ||
			l.ReportOutputBytes <= 0 || l.ReportNodes <= 0 || l.ReportDepth <= 0 ||
			p.SourceByteLength > l.SourceBytes || int64(len(p.Parts)) > l.PartCount {
			return ErrLinkage
		}
		total := int64(0)
		for _, part := range p.Parts {
			if part.ByteLength != nil {
				if *part.ByteLength < 0 || *part.ByteLength > l.PartBytes || *part.ByteLength > l.AggregateBytes-total {
					return ErrLinkage
				}
				total += *part.ByteLength
			}
		}
	}
	for _, s := range x.Scopes {
		part := x.Parts[s.PartRef]
		p := x.Packages[part.PackageRef]
		if int64(len(s.Text)) > p.Limits.MaxScopeTextUTF8Bytes || int64(len(s.Origins)) > p.Limits.MaxScopeScalarOrigins {
			return ErrLinkage
		}
		var removed strings.Builder
		for _, r := range s.Text {
			if !analyzers.Removable(r) {
				removed.WriteRune(r)
			}
		}
		if s.Hashes.NfcTextSHA256 != identity.ExactBytes([]byte(u.Normalize(s.Text, false))) ||
			s.Hashes.NfkcTextSHA256 != identity.ExactBytes([]byte(u.Normalize(s.Text, true))) ||
			s.Hashes.FormattingRemovedSHA256 != identity.ExactBytes([]byte(removed.String())) {
			return ErrLinkage
		}
	}
	return nil
}
