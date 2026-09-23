package opcrels

import (
	"context"
	"reflect"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
)

func TestSessionSharesParsingAcrossAdmissionBarriers(t *testing.T) {
	raw := archive(t, map[string]string{"_rels/.rels": rels(relationship("r1", "word/main.xml", "")), "word/main.xml": "<a/>", "word/_rels/main.xml.rels": rels(relationship("r2", "header.xml", "")), "word/header.xml": "<b/>"})
	reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Admit(context.Background(), []string{"_rels/.rels"}); err != nil {
		t.Fatal(err)
	}
	if err := session.Parse(context.Background(), []string{"_rels/.rels", "word/_rels/main.xml.rels"}); err != nil {
		t.Fatal(err)
	}
	first, err := session.Result(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Relationships) != 1 || first.Relationships[0].TargetCode != "office.part_not_admitted" || first.Relationships[0].TargetSHA256 != "" {
		t.Fatal("invented target verification")
	}
	rootXML := session.parsed["_rels/.rels"].XML
	counters := [4]int{session.bytesLeft, session.tokensLeft, session.valuesLeft, session.partsLeft}
	if err := session.Parse(context.Background(), []string{"_rels/.rels"}); err != nil {
		t.Fatal(err)
	}
	if [4]int{session.bytesLeft, session.tokensLeft, session.valuesLeft, session.partsLeft} != counters {
		t.Fatal("reparsed root declaration")
	}
	if err := reader.Admit(context.Background(), []string{"word/main.xml", "word/_rels/main.xml.rels", "word/header.xml"}); err != nil {
		t.Fatal(err)
	}
	if err := session.Parse(context.Background(), []string{"word/_rels/main.xml.rels"}); err != nil {
		t.Fatal(err)
	}
	final, err := session.Result(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if final.State != "completed" || len(final.Relationships) != 2 || session.parsed["_rels/.rels"].XML != rootXML {
		t.Fatal("lost cached declaration")
	}
	for _, r := range final.Relationships {
		if r.State != "resolved" || r.TargetSHA256 == "" {
			t.Fatal("missing verified target")
		}
	}
	legacy, err := Inspect(context.Background(), raw, evidence.Hash(raw))
	if err != nil {
		t.Fatal(err)
	}
	final.Outcomes = nil
	legacy.Outcomes = nil
	legacy.Package = nil
	if !reflect.DeepEqual(final, legacy) {
		t.Fatal("staged inventory differs from completed legacy inventory")
	}
}

func TestSessionDoesNotParseOrdinaryPartsOrReplayFailedXML(t *testing.T) {
	raw := archive(t, map[string]string{"_rels/.rels": "<broken>", "ordinary.xml": "<a/>"})
	reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Admit(context.Background(), []string{"_rels/.rels", "ordinary.xml"}); err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Parse(context.Background(), []string{"ordinary.xml", "_rels/.rels"}); err != nil {
		t.Fatal(err)
	}
	if len(session.parsed) != 1 || session.parsed["_rels/.rels"].State != "failed" {
		t.Fatal("incorrect parse inventory")
	}
	counters := [4]int{session.bytesLeft, session.tokensLeft, session.valuesLeft, session.partsLeft}
	if err := session.Parse(context.Background(), []string{"_rels/.rels"}); err != nil {
		t.Fatal(err)
	}
	if [4]int{session.bytesLeft, session.tokensLeft, session.valuesLeft, session.partsLeft} != counters {
		t.Fatal("retried failed parse")
	}
	r, err := session.Result(context.Background())
	if err != nil || r.State != "partial" || r.Parts[0].XML != nil {
		t.Fatal("lost parse failure", err)
	}
}

func TestMalformedPartChargesConsumptionAcrossBothPaths(t *testing.T) {
	raw := archive(t, map[string]string{"_rels/.rels": "<broken>", "word/main.xml": "<r/>", "word/_rels/main.xml.rels": rels(relationship("later", "target.bin", "")), "word/target.bin": "payload"})
	reader, err := packageparts.OpenOutcomes(context.Background(), raw, evidence.Hash(raw), packageparts.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, part := range reader.View().Parts {
		names = append(names, part.Part.Name)
	}
	if err := reader.Admit(context.Background(), names); err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Parse(context.Background(), []string{"_rels/.rels"}); err != nil {
		t.Fatal(err)
	}
	if session.tokensLeft != maxTokens-1 || session.valuesLeft <= 0 {
		t.Fatal("failed parse charged reservation", session.tokensLeft, session.valuesLeft)
	}
	if err := session.Parse(context.Background(), []string{"word/_rels/main.xml.rels"}); err != nil {
		t.Fatal(err)
	}
	staged, err := session.Result(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	strict, err := Inspect(context.Background(), raw, evidence.Hash(raw))
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range []*Result{staged, strict} {
		if result.State != "partial" || len(result.Relationships) != 1 || result.Relationships[0].ID != "later" {
			t.Fatal("unrelated evidence suppressed", result)
		}
		for _, part := range result.Parts {
			if part.Part == "word/_rels/main.xml.rels" && part.State != "completed" {
				t.Fatal("invented resource failure", part)
			}
		}
	}
}
