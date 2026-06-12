package do

import "testing"

func TestAlumniDedupKeyIncludesMobile(t *testing.T) {
	base := AlumniDedupKey{Name: "张三", Grade: "2020级", ClassName: "MPA", Cohort: "2020"}
	withMobileA := base
	withMobileA.Mobile = "13800000000"
	withMobileB := base
	withMobileB.Mobile = "13900000000"

	if withMobileA.Key() == withMobileB.Key() {
		t.Fatalf("expected different keys for different mobile values, got %q", withMobileA.Key())
	}
}

func TestAlumniDedupKeyTrimsFields(t *testing.T) {
	trimmed := AlumniDedupKey{Name: "张三", Grade: "2020级", ClassName: "MPA", Cohort: "2020", Mobile: "13800000000"}
	padded := AlumniDedupKey{Name: " 张三 ", Grade: " 2020级 ", ClassName: " MPA ", Cohort: " 2020 ", Mobile: " 13800000000 "}

	if trimmed.Key() != padded.Key() {
		t.Fatalf("expected padded key %q to match trimmed key %q", padded.Key(), trimmed.Key())
	}
}

func TestAlumniCreateProfileNormalizeTrimsMobile(t *testing.T) {
	mobile := " 13800000000 "
	profile := AlumniCreateProfile{Name: "张三", Grade: "2020级", Mobile: &mobile}.Normalize()

	if profile.Mobile == nil || *profile.Mobile != "13800000000" {
		t.Fatalf("expected trimmed mobile, got %+v", profile.Mobile)
	}
}

func TestAlumniCreateProfileNormalizeEmptyMobile(t *testing.T) {
	mobile := " "
	profile := AlumniCreateProfile{Name: "张三", Grade: "2020级", Mobile: &mobile}.Normalize()

	if profile.Mobile != nil {
		t.Fatalf("expected empty mobile to be nil, got %+v", profile.Mobile)
	}
}
