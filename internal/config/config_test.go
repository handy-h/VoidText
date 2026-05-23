package config

import (
	"os"
	"testing"
)

func TestLoad_ShouldReturnNoError(t *testing.T) {
	err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoad_ShouldSetDefaultPort(t *testing.T) {
	os.Unsetenv("PORT")
	Load()

	if AppConfigInstance.Port != 8080 {
		t.Errorf("Default Port = %d, want 8080", AppConfigInstance.Port)
	}
}

func TestLoad_ShouldOverridePortFromEnv(t *testing.T) {
	os.Setenv("PORT", "9090")
	defer os.Unsetenv("PORT")

	Load()

	if AppConfigInstance.Port != 9090 {
		t.Errorf("Port = %d, want 9090", AppConfigInstance.Port)
	}
}

func TestLoad_ShouldOverrideBoolFromEnv(t *testing.T) {
	os.Setenv("ENABLE_BASIC_CLEANING", "false")
	defer os.Unsetenv("ENABLE_BASIC_CLEANING")

	Load()

	if AppConfigInstance.EnableBasicCleaning != false {
		t.Errorf("EnableBasicCleaning = %v, want false", AppConfigInstance.EnableBasicCleaning)
	}
}

func TestLoad_ShouldOverrideInt64FromEnv(t *testing.T) {
	os.Setenv("MAX_FILE_SIZE", "1024")
	defer os.Unsetenv("MAX_FILE_SIZE")

	Load()

	if AppConfigInstance.MaxFileSize != 1024 {
		t.Errorf("MaxFileSize = %d, want 1024", AppConfigInstance.MaxFileSize)
	}
}

func TestLoad_ShouldOverrideFloatFromEnv(t *testing.T) {
	os.Setenv("VECTOR_SIMILARITY_THRESHOLD", "0.85")
	defer os.Unsetenv("VECTOR_SIMILARITY_THRESHOLD")

	Load()

	if AppConfigInstance.VectorSimilarityThreshold != 0.85 {
		t.Errorf("VectorSimilarityThreshold = %f, want 0.85", AppConfigInstance.VectorSimilarityThreshold)
	}
}

func TestGetNameSeparators_ShouldReturnSeparatorList(t *testing.T) {
	AppConfigInstance.NameSeparators = "-|—|·|_"
	separators := AppConfigInstance.GetNameSeparators()

	if len(separators) != 4 {
		t.Errorf("GetNameSeparators() length = %d, want 4", len(separators))
	}
}

func TestGetNameSeparators_ShouldFilterEmptyParts(t *testing.T) {
	AppConfigInstance.NameSeparators = "-| |—|"
	separators := AppConfigInstance.GetNameSeparators()

	for _, sep := range separators {
		if sep == "" {
			t.Errorf("GetNameSeparators() should not contain empty strings")
		}
	}
}

func TestValidate_ShouldPassForValidConfig(t *testing.T) {
	Load()

	err := Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_ShouldFailForInvalidPort(t *testing.T) {
	AppConfigInstance.Port = 0

	err := Validate()
	if err == nil {
		t.Errorf("Validate() should fail for port 0")
	}
}

func TestValidate_ShouldFailForInvalidSimilarityThreshold(t *testing.T) {
	AppConfigInstance.VectorSimilarityThreshold = 1.5

	err := Validate()
	if err == nil {
		t.Errorf("Validate() should fail for similarity threshold > 1")
	}
}

func TestValidate_ShouldFailForInvalidTemperature(t *testing.T) {
	AppConfigInstance.CompletionTemperature = 3.0

	err := Validate()
	if err == nil {
		t.Errorf("Validate() should fail for temperature > 2")
	}
}

func TestLoad_ShouldSetDefaultLLMConcurrency(t *testing.T) {
	os.Unsetenv("LLM_CONCURRENCY")
	Load()

	if AppConfigInstance.LLMConcurrency != 2 {
		t.Errorf("LLMConcurrency = %d, want 2", AppConfigInstance.LLMConcurrency)
	}
}

func TestLoad_ShouldOverrideLLMConcurrencyFromEnv(t *testing.T) {
	os.Setenv("LLM_CONCURRENCY", "5")
	defer os.Unsetenv("LLM_CONCURRENCY")

	Load()

	if AppConfigInstance.LLMConcurrency != 5 {
		t.Errorf("LLMConcurrency = %d, want 5", AppConfigInstance.LLMConcurrency)
	}
}

func TestGetEnvStr_ShouldReturnFallback(t *testing.T) {
	os.Unsetenv("NONEXISTENT_KEY_TEST")
	result := getEnvStr("NONEXISTENT_KEY_TEST", "fallback")
	if result != "fallback" {
		t.Errorf("getEnvStr() = %s, want fallback", result)
	}
}

func TestGetEnvInt_ShouldReturnFallbackForInvalidValue(t *testing.T) {
	os.Setenv("TEST_INVALID_INT", "not_a_number")
	defer os.Unsetenv("TEST_INVALID_INT")

	result := getEnvInt("TEST_INVALID_INT", 42)
	if result != 42 {
		t.Errorf("getEnvInt() = %d, want 42", result)
	}
}

func TestGetEnvBool_ShouldReturnFallbackForInvalidValue(t *testing.T) {
	os.Setenv("TEST_INVALID_BOOL", "not_a_bool")
	defer os.Unsetenv("TEST_INVALID_BOOL")

	result := getEnvBool("TEST_INVALID_BOOL", true)
	if result != true {
		t.Errorf("getEnvBool() = %v, want true", result)
	}
}

func TestGetEnvFloat_ShouldReturnFallbackForInvalidValue(t *testing.T) {
	os.Setenv("TEST_INVALID_FLOAT", "not_a_float")
	defer os.Unsetenv("TEST_INVALID_FLOAT")

	result := getEnvFloat("TEST_INVALID_FLOAT", 0.5)
	if result != 0.5 {
		t.Errorf("getEnvFloat() = %f, want 0.5", result)
	}
}
