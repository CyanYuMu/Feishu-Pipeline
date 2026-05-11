package pipeline

import "encoding/json"

func PreviousExecutableStage(stageKey string) string {
	switch stageKey {
	case StageCheckpointDesign:
		return StageSolutionDesign
	case StageCheckpointReview:
		return StageCodeReview
	default:
		return ""
	}
}

func NextStagesForReset(stageKey string) []string {
	switch stageKey {
	case StageSolutionDesign:
		return []string{StageCheckpointDesign, StageCodeGeneration, StageTestGeneration, StageTestExecution, StageCodeReview, StageCheckpointReview, StageDelivery}
	case StageCodeReview:
		return []string{StageCheckpointReview, StageDelivery}
	default:
		return nil
	}
}

func ShouldStageWaitForApproval(stageKey string) bool {
	return stageKey == StageCheckpointDesign || stageKey == StageCheckpointReview
}

func NextStage(stageKey string) string {
	for idx, stage := range DefaultStageDefinitions {
		if stage.Key == stageKey && idx+1 < len(DefaultStageDefinitions) {
			return DefaultStageDefinitions[idx+1].Key
		}
	}
	return ""
}

func BuildRejectContext(comment string, previousOutput string) string {
	payload := map[string]any{"rejectReason": comment}
	if previousOutput != "" {
		payload["previousOutput"] = previousOutput
	}
	bytes, _ := json.Marshal(payload)
	return string(bytes)
}
