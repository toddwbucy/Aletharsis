package odtidentify

import (
	"context"
	"github.com/toddwbucy/Aletharsis/internal/packageparts"
	"testing"
)

func TestFinishPackageRejectsMissingManifest(t *testing.T) {
	for _, parts := range [][]packageparts.Outcome{nil, {{Part: packageparts.Part{Name: "unrelated.xml"}}}} {
		if _, err := finishPackage(context.Background(), &packageparts.OutcomeView{Parts: parts}, &Result{}); err != packageparts.ErrIdentity {
			t.Fatal("missing manifest accepted", err)
		}
	}
}
