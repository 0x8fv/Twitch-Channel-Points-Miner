package constants

import "testing"

func TestColorConstantsNonEmpty(t *testing.T) {
	if ColorGreen == "" || ColorRed == "" || ColorCyan == "" || ColorPurple == "" || ColorYellow == "" || ColorReset == "" {
		t.Fatalf("color constants should not be empty")
	}
}

func TestGQLOperationsPresence(t *testing.T) {
	if GQLOperations.URL == "" || GQLOperations.IntegrityURL == "" {
		t.Fatalf("gql urls should be set")
	}
	if GQLOperations.WithIsStreamLiveQuery.OperationName == "" || GQLOperations.ChannelPointsContext.Extensions.PersistedQuery.Sha256Hash == "" {
		t.Fatalf("persisted operations should have names and hashes")
	}
	if GQLOperations.RewardList.OperationName == "" || GQLOperations.RewardList.Extensions.PersistedQuery.Sha256Hash == "" {
		t.Fatalf("reward list operation should be populated")
	}
	if len(GQLOperations.PersonalSections) == 0 {
		t.Fatalf("expected personal sections populated")
	}
}
