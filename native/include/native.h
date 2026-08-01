/**************************************************************************
 * DiffScope Synthesis Platform                                           *
 * Copyright (C) 2026 Team OpenVPI                                        *
 *                                                                        *
 * This program is free software: you can redistribute it and/or modify   *
 * it under the terms of the GNU General Public License as published by   *
 * the Free Software Foundation, either version 3 of the License, or      *
 * (at your option) any later version.                                    *
 *                                                                        *
 * This program is distributed in the hope that it will be useful,        *
 * but WITHOUT ANY WARRANTY; without even the implied warranty of         *
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the          *
 * GNU General Public License for more details.                           *
 *                                                                        *
 * You should have received a copy of the GNU General Public License      *
 * along with this program.  If not, see <https://www.gnu.org/licenses/>. *
 **************************************************************************/

#ifndef DSSP_NATIVE_H
#define DSSP_NATIVE_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ========================================================================
 * Logging
 * ====================================================================== */

typedef void (*DSSP_LogCallback)(const char *component, int level, const char *message);

void DSSP_SetLogCallback(DSSP_LogCallback log_callback);

/* ========================================================================
 * Execution Provider Info
 * ====================================================================== */

typedef enum DSSP_ExecutionProvider {
	DSSP_ExecutionProvider_CPU,
	DSSP_ExecutionProvider_CUDA,
	DSSP_ExecutionProvider_DirectML,
	DSSP_ExecutionProvider_CoreML,
} DSSP_ExecutionProvider;

typedef const void *DSSP_Device;

DSSP_Device DSSP_GetDefaultDevice(void);
DSSP_ExecutionProvider DSSP_GetDeviceExecutionProvider(DSSP_Device device);
int DSSP_GetDeviceIndex(DSSP_Device device);
const char *DSSP_GetDeviceDescription(DSSP_Device device);
const char *DSSP_GetDeviceID(DSSP_Device device);
uint64_t DSSP_GetDeviceMemory(DSSP_Device device);
bool DSSP_HasExecutionProvider(DSSP_ExecutionProvider execution_provider);
size_t DSSP_GetExecutionProviderDeviceCount(DSSP_ExecutionProvider execution_provider);
DSSP_Device DSSP_GetExecutionProviderDevice(DSSP_ExecutionProvider execution_provider, size_t index);

/* ========================================================================
 * Language Conversion
 * ====================================================================== */

typedef void *DSSP_Lyrics;

DSSP_Lyrics DSSP_AllocateLyrics(size_t count);
void DSSP_FreeLyrics(DSSP_Lyrics lyrics);
size_t DSSP_GetLyricCount(DSSP_Lyrics lyrics);
void DSSP_SetLyricText(DSSP_Lyrics lyrics, size_t index, const char *text);
void DSSP_SetLyricLanguage(DSSP_Lyrics lyrics, size_t index, const char *language);
const char *DSSP_GetLyricText(DSSP_Lyrics lyrics, size_t index);
const char *DSSP_GetLyricLanguage(DSSP_Lyrics lyrics, size_t index);

typedef void *DSSP_Pronunciations;

void DSSP_FreePronunciations(DSSP_Pronunciations pronunciations);
size_t DSSP_GetPronunciationCount(DSSP_Pronunciations pronunciations);
const char *DSSP_GetPronunciationText(DSSP_Pronunciations pronunciations, size_t index);
size_t DSSP_GetPronunciationCandidateCount(DSSP_Pronunciations pronunciations, size_t index);
const char *DSSP_GetPronunciationCandidate(DSSP_Pronunciations pronunciations, size_t index, size_t candidate_index);
bool DSSP_IsPronunciationError(DSSP_Pronunciations pronunciations, size_t index);

bool DSSP_InitializeLanguageConversion(DSSP_Device device);
const char *DSSP_GetLanguageConversionErrorMessage(void);
DSSP_Pronunciations DSSP_ConvertLanguage(DSSP_Lyrics lyrics);

/* ========================================================================
 * Phoneme Conversion
 * ====================================================================== */

typedef void *DSSP_Phonemes;

void DSSP_FreePhonemes(DSSP_Phonemes phonemes);
size_t DSSP_GetPhonemeCount(DSSP_Phonemes phonemes);
const char *DSSP_GetPhonemeText(DSSP_Phonemes phonemes, size_t index);
bool DSSP_IsPhonemeOnset(DSSP_Phonemes phonemes, size_t index);

typedef void *DSSP_S2P;

DSSP_S2P DSSP_NewDirectS2P(void);
DSSP_S2P DSSP_NewMapS2P(const char *mapping_file_path);
DSSP_S2P DSSP_NewDictS2P(const char *dictionary_file_path);
DSSP_S2P DSSP_NewCustomS2P(const char *lua_script_file_path);
void DSSP_DeleteS2P(DSSP_S2P s2p);
bool DSSP_IsS2PError(DSSP_S2P s2p);
const char *DSSP_GetS2PErrorMessage(DSSP_S2P s2p);

// thread-safe
DSSP_Phonemes DSSP_RunS2P(DSSP_S2P s2p, const char *pronunciation_text);
void DSSP_TerminateCustomS2P(DSSP_S2P s2p);

typedef void *DSSP_OnsetMarker;

DSSP_OnsetMarker DSSP_NewRuleOnsetMarker(const char *rule_file_path);
DSSP_OnsetMarker DSSP_NewCustomOnsetMarker(const char *lua_script_file_path);
void DSSP_DeleteOnsetMarker(DSSP_OnsetMarker onset_marker);
bool DSSP_IsOnsetMarkerError(DSSP_OnsetMarker onset_marker);
const char *DSSP_GetOnsetMarkerErrorMessage(DSSP_OnsetMarker onset_marker);

// thread-safe
void DSSP_RunOnsetMarker(DSSP_OnsetMarker onset_marker, DSSP_Phonemes phonemes);
void DSSP_TerminateCustomOnsetMarker(DSSP_OnsetMarker onset_marker);

void DSSP_SetLuaRunnerCount(size_t count);

/* ========================================================================
 * SynthRT
 * ====================================================================== */

bool DSSP_InitializeSynthRT(const char *package_path, DSSP_Device device);
const char *DSSP_GetSynthRTErrorMessage(void);

typedef struct DSSP_SRTVersionNumber {
	int major, minor, patch, tweak;
} DSSP_SRTVersionNumber;

typedef void *DSSP_SRTPackage;

// Nullable: indicates error
DSSP_SRTPackage DSSP_GetSRTPackage(const char *package_dir, const char *package_id, DSSP_SRTVersionNumber versionNumber);

typedef void *DSSP_SRTSinger;

// Nullable: indicates error
DSSP_SRTSinger DSSP_GetSRTSinger(DSSP_SRTPackage package, const char *singer_id);

/* ========================================================================
 * dsinfer (data)
 * ====================================================================== */

// Note:
// All `Free` functions will free both the allocated memory and the content. For example, `DSSP_FreeDiffSingerWords` will free all words, phonemes, notes, and managed double arrays.
// `Set` functions transfer ownership of the content to the target structure. For example, `DSSP_SetDiffSingerWordPhonemes` will transfer ownership of the phonemes to the word, and the caller should not free or access the phonemes after calling this function.

typedef void *DSSP_DiffSingerManagedDoubleArray;

DSSP_DiffSingerManagedDoubleArray DSSP_AllocateDiffSingerManagedDoubleArray(size_t count);
void DSSP_FreeDiffSingerManagedDoubleArray(DSSP_DiffSingerManagedDoubleArray array);
size_t DSSP_GetDiffSingerManagedDoubleArrayCount(DSSP_DiffSingerManagedDoubleArray array);
double *DSSP_GetDiffSingerManagedDoubleArrayData(DSSP_DiffSingerManagedDoubleArray array);

typedef void *DSSP_DiffSingerSpeakers;

DSSP_DiffSingerSpeakers DSSP_AllocateDiffSingerSpeakers(size_t count);
void DSSP_FreeDiffSingerSpeakers(DSSP_DiffSingerSpeakers speakers);
size_t DSSP_GetDiffSingerSpeakerCount(DSSP_DiffSingerSpeakers speakers);
const char *DSSP_GetDiffSingerSpeakerID(DSSP_DiffSingerSpeakers speakers, size_t index);
void DSSP_SetDiffSingerSpeakerID(DSSP_DiffSingerSpeakers speakers, size_t index, const char *speakerID);
double DSSP_GetDiffSingerSpeakerProportion(DSSP_DiffSingerSpeakers speakers, size_t index);
void DSSP_SetDiffSingerSpeakerProportion(DSSP_DiffSingerSpeakers speakers, size_t index, double proportion);

typedef void *DSSP_DiffSingerDynamicMixedSpeakers;

DSSP_DiffSingerDynamicMixedSpeakers DSSP_AllocateDiffSingerDynamicMixedSpeakers(size_t count);
void DSSP_FreeDiffSingerDynamicMixedSpeakers(DSSP_DiffSingerDynamicMixedSpeakers speakers);
size_t DSSP_GetDiffSingerDynamicMixedSpeakerCount(DSSP_DiffSingerDynamicMixedSpeakers speakers);
const char *DSSP_GetDiffSingerDynamicMixedSpeakerID(DSSP_DiffSingerDynamicMixedSpeakers speakers, size_t index);
void DSSP_SetDiffSingerDynamicMixedSpeakerID(DSSP_DiffSingerDynamicMixedSpeakers speakers, size_t index, const char *speakerID);
DSSP_DiffSingerManagedDoubleArray DSSP_GetDiffSingerDynamicMixedSpeakerProportions(DSSP_DiffSingerDynamicMixedSpeakers speakers, size_t index);
void DSSP_SetDiffSingerDynamicMixedSpeakerProportions(DSSP_DiffSingerDynamicMixedSpeakers speakers, size_t index, DSSP_DiffSingerManagedDoubleArray proportions);
double DSSP_GetDiffSingerDynamicMixedSpeakerInterval(DSSP_DiffSingerDynamicMixedSpeakers speakers, size_t index);
void DSSP_SetDiffSingerDynamicMixedSpeakerInterval(DSSP_DiffSingerDynamicMixedSpeakers speakers, size_t index, double interval);

typedef void *DSSP_DiffSingerPhonemes;

DSSP_DiffSingerPhonemes DSSP_AllocateDiffSingerPhonemes(size_t count);
void DSSP_FreeDiffSingerPhonemes(DSSP_DiffSingerPhonemes phonemes);
size_t DSSP_GetDiffSingerPhonemeCount(DSSP_DiffSingerPhonemes phonemes);
const char *DSSP_GetDiffSingerPhonemeToken(DSSP_DiffSingerPhonemes phonemes, size_t index);
void DSSP_SetDiffSingerPhonemeToken(DSSP_DiffSingerPhonemes phonemes, size_t index, const char *token);
const char *DSSP_GetDiffSingerPhonemeLanguage(DSSP_DiffSingerPhonemes phonemes, size_t index);
void DSSP_SetDiffSingerPhonemeLanguage(DSSP_DiffSingerPhonemes phonemes, size_t index, const char *language);
double DSSP_GetDiffSingerPhonemeStart(DSSP_DiffSingerPhonemes phonemes, size_t index);
void DSSP_SetDiffSingerPhonemeStart(DSSP_DiffSingerPhonemes phonemes, size_t index, double start);
DSSP_DiffSingerSpeakers DSSP_GetDiffSingerPhonemeSpeakers(DSSP_DiffSingerPhonemes phonemes, size_t index);
void DSSP_SetDiffSingerPhonemeSpeakers(DSSP_DiffSingerPhonemes phonemes, size_t index, DSSP_DiffSingerSpeakers speakers);

typedef void *DSSP_DiffSingerNotes;

DSSP_DiffSingerNotes DSSP_AllocateDiffSingerNotes(size_t count);
void DSSP_FreeDiffSingerNotes(DSSP_DiffSingerNotes notes);
size_t DSSP_GetDiffSingerNoteCount(DSSP_DiffSingerNotes notes);
int DSSP_GetDiffSingerNoteCent(DSSP_DiffSingerNotes notes, size_t index);
void DSSP_SetDiffSingerNoteCent(DSSP_DiffSingerNotes notes, size_t index, int cent);
double DSSP_GetDiffSingerNoteDuration(DSSP_DiffSingerNotes notes, size_t index);
void DSSP_SetDiffSingerNoteDuration(DSSP_DiffSingerNotes notes, size_t index, double duration);
bool DSSP_IsDiffSingerNoteRest(DSSP_DiffSingerNotes notes, size_t index);
void DSSP_SetDiffSingerNoteRest(DSSP_DiffSingerNotes notes, size_t index, bool isRest);

typedef void *DSSP_DiffSingerWords;

DSSP_DiffSingerWords DSSP_AllocateDiffSingerWords(size_t count);
void DSSP_FreeDiffSingerWords(DSSP_DiffSingerWords words);
size_t DSSP_GetDiffSingerWordCount(DSSP_DiffSingerWords words);
DSSP_DiffSingerPhonemes DSSP_GetDiffSingerWordPhonemes(DSSP_DiffSingerWords words, size_t index);
void DSSP_SetDiffSingerWordPhonemes(DSSP_DiffSingerWords words, size_t index, DSSP_DiffSingerPhonemes phonemes);
DSSP_DiffSingerNotes DSSP_GetDiffSingerWordNotes(DSSP_DiffSingerWords words, size_t index);
void DSSP_SetDiffSingerWordNotes(DSSP_DiffSingerWords words, size_t index, DSSP_DiffSingerNotes notes);

typedef void *DSSP_DiffSingerParameters;

typedef enum DSSP_DiffSingerParameterTag {
	DSSP_DiffSingerParameterTag_Pitch,
	DSSP_DiffSingerParameterTag_Expr,
	DSSP_DiffSingerParameterTag_F0,
	DSSP_DiffSingerParameterTag_ToneShift,
	DSSP_DiffSingerParameterTag_Energy,
	DSSP_DiffSingerParameterTag_Breathiness,
	DSSP_DiffSingerParameterTag_Voicing,
	DSSP_DiffSingerParameterTag_Tension,
	DSSP_DiffSingerParameterTag_MouthOpening,
	DSSP_DiffSingerParameterTag_Gender,
	DSSP_DiffSingerParameterTag_Velocity,
} DSSP_DiffSingerParameterTag;

DSSP_DiffSingerParameters DSSP_AllocateDiffSingerParameters(size_t count);
void DSSP_FreeDiffSingerParameters(DSSP_DiffSingerParameters parameters);
size_t DSSP_GetDiffSingerParameterCount(DSSP_DiffSingerParameters parameters);
DSSP_DiffSingerParameterTag DSSP_GetDiffSingerParameterTag(DSSP_DiffSingerParameters parameters, size_t index);
void DSSP_SetDiffSingerParameterTag(DSSP_DiffSingerParameters parameters, size_t index, DSSP_DiffSingerParameterTag parameterName);
DSSP_DiffSingerManagedDoubleArray DSSP_GetDiffSingerParameterValues(DSSP_DiffSingerParameters parameters, size_t index);
void DSSP_SetDiffSingerParameterValues(DSSP_DiffSingerParameters parameters, size_t index, DSSP_DiffSingerManagedDoubleArray values);
double DSSP_GetDiffSingerParameterInterval(DSSP_DiffSingerParameters parameters, size_t index);
void DSSP_SetDiffSingerParameterInterval(DSSP_DiffSingerParameters parameters, size_t index, double interval);
bool DSSP_IsDiffSingerParameterRetake(DSSP_DiffSingerParameters parameters, size_t index);
void DSSP_SetDiffSingerParameterRetake(DSSP_DiffSingerParameters parameters, size_t index, bool isRetake);
double DSSP_GetDiffSingerParameterRetakeStart(DSSP_DiffSingerParameters parameters, size_t index);
void DSSP_SetDiffSingerParameterRetakeStart(DSSP_DiffSingerParameters parameters, size_t index, double retakeStart);
double DSSP_GetDiffSingerParameterRetakeLength(DSSP_DiffSingerParameters parameters, size_t index);
void DSSP_SetDiffSingerParameterRetakeLength(DSSP_DiffSingerParameters parameters, size_t index, double retakeLength);

typedef void *DSSP_DiffSingerAcousticFeature;

void DSSP_DeleteDiffSingerAcousticFeature(DSSP_DiffSingerAcousticFeature feature);

typedef void *DSSP_DiffSingerAudioData;

void DSSP_DeleteDiffSingerAudioData(DSSP_DiffSingerAudioData audioData);

typedef void *DSSP_DiffSingerRawData;

void DSSP_FreeDiffSingerRawData(DSSP_DiffSingerRawData rawData);
size_t DSSP_GetDiffSingerRawDataSize(DSSP_DiffSingerRawData rawData);
const uint8_t *DSSP_GetDiffSingerRawDataBytes(DSSP_DiffSingerRawData rawData);

/* ========================================================================
 * dsinfer (inference)
 * ====================================================================== */

const char *DSSP_GetDiffSingerInferenceDigestKey(DSSP_SRTSinger singer);

typedef void *DSSP_DiffSingerDurationInference;

DSSP_DiffSingerDurationInference DSSP_GetDiffSingerDurationInference(DSSP_SRTSinger singer);
const char *DSSP_GetDiffSingerDurationInferenceSpeakerID(DSSP_SRTSinger singer, const char *singer_speaker_id);

typedef void *DSSP_DiffSingerDurationInferenceTask;

DSSP_DiffSingerDurationInferenceTask DSSP_CreateDiffSingerDurationInferenceTask(DSSP_DiffSingerDurationInference inference);
void DSSP_DeleteDiffSingerDurationInferenceTask(DSSP_DiffSingerDurationInferenceTask task);
bool DSSP_IsDiffSingerDurationInferenceTaskError(DSSP_DiffSingerDurationInferenceTask task);
const char *DSSP_GetDiffSingerDurationInferenceErrorMessage(DSSP_DiffSingerDurationInferenceTask task);
DSSP_DiffSingerDurationInference DSSP_GetDiffSingerDurationInferenceTaskInference(DSSP_DiffSingerDurationInferenceTask task);

// Nullable: indicates error
// Reentrant but not thread-safe
DSSP_DiffSingerManagedDoubleArray DSSP_RunDiffSingerDurationInferenceTask(DSSP_DiffSingerDurationInferenceTask task, double duration, DSSP_DiffSingerWords words);

// thread-safe
void DSSP_TerminateDiffSingerDurationInferenceTask(DSSP_DiffSingerDurationInferenceTask task);

typedef void *DSSP_DiffSingerPitchInference;

DSSP_DiffSingerPitchInference DSSP_GetDiffSingerPitchInference(DSSP_SRTSinger singer);
const char *DSSP_GetDiffSingerPitchInferenceSpeakerID(DSSP_SRTSinger singer, const char *singer_speaker_id);

typedef void *DSSP_DiffSingerPitchInferenceTask;

DSSP_DiffSingerPitchInferenceTask DSSP_CreateDiffSingerPitchInferenceTask(DSSP_DiffSingerPitchInference inference);
void DSSP_DeleteDiffSingerPitchInferenceTask(DSSP_DiffSingerPitchInferenceTask task);
bool DSSP_IsDiffSingerPitchInferenceTaskError(DSSP_DiffSingerPitchInferenceTask task);
const char *DSSP_GetDiffSingerPitchInferenceErrorMessage(DSSP_DiffSingerPitchInferenceTask task);
DSSP_DiffSingerPitchInference DSSP_GetDiffSingerPitchInferenceTaskInference(DSSP_DiffSingerPitchInferenceTask task);

// Nullable: indicates error
// Reentrant but not thread-safe
DSSP_DiffSingerParameters DSSP_RunDiffSingerPitchInferenceTask(DSSP_DiffSingerPitchInferenceTask task, double duration, DSSP_DiffSingerWords words, DSSP_DiffSingerParameters parameters, DSSP_DiffSingerDynamicMixedSpeakers dynamicMixedSpeakers, int64_t steps);

// thread-safe
void DSSP_TerminateDiffSingerPitchInferenceTask(DSSP_DiffSingerPitchInferenceTask task);

typedef void *DSSP_DiffSingerVarianceInference;

DSSP_DiffSingerVarianceInference DSSP_GetDiffSingerVarianceInference(DSSP_SRTSinger singer);
const char *DSSP_GetDiffSingerVarianceInferenceSpeakerID(DSSP_SRTSinger singer, const char *singer_speaker_id);

typedef void *DSSP_DiffSingerVarianceInferenceTask;

DSSP_DiffSingerVarianceInferenceTask DSSP_CreateDiffSingerVarianceInferenceTask(DSSP_DiffSingerVarianceInference inference);
void DSSP_DeleteDiffSingerVarianceInferenceTask(DSSP_DiffSingerVarianceInferenceTask task);
bool DSSP_IsDiffSingerVarianceInferenceTaskError(DSSP_DiffSingerVarianceInferenceTask task);
const char *DSSP_GetDiffSingerVarianceInferenceErrorMessage(DSSP_DiffSingerVarianceInferenceTask task);
DSSP_DiffSingerVarianceInference DSSP_GetDiffSingerVarianceInferenceTaskInference(DSSP_DiffSingerVarianceInferenceTask task);

// Nullable: indicates error
// Reentrant but not thread-safe
DSSP_DiffSingerParameters DSSP_RunDiffSingerVarianceInferenceTask(DSSP_DiffSingerVarianceInferenceTask task, double duration, DSSP_DiffSingerWords words, DSSP_DiffSingerParameters parameters, DSSP_DiffSingerDynamicMixedSpeakers dynamicMixedSpeakers, int64_t steps);

// thread-safe
void DSSP_TerminateDiffSingerVarianceInferenceTask(DSSP_DiffSingerVarianceInferenceTask task);

typedef void *DSSP_DiffSingerAcousticInference;

DSSP_DiffSingerAcousticInference DSSP_GetDiffSingerAcousticInference(DSSP_SRTSinger singer);
const char *DSSP_GetDiffSingerAcousticInferenceSpeakerID(DSSP_SRTSinger singer, const char *singer_speaker_id);

typedef void *DSSP_DiffSingerAcousticInferenceTask;

DSSP_DiffSingerAcousticInferenceTask DSSP_CreateDiffSingerAcousticInferenceTask(DSSP_DiffSingerAcousticInference inference);
void DSSP_DeleteDiffSingerAcousticInferenceTask(DSSP_DiffSingerAcousticInferenceTask task);
bool DSSP_IsDiffSingerAcousticInferenceTaskError(DSSP_DiffSingerAcousticInferenceTask task);
const char *DSSP_GetDiffSingerAcousticInferenceErrorMessage(DSSP_DiffSingerAcousticInferenceTask task);
DSSP_DiffSingerAcousticInference DSSP_GetDiffSingerAcousticInferenceTaskInference(DSSP_DiffSingerAcousticInferenceTask task);

// Nullable: indicates error
// Reentrant but not thread-safe
DSSP_DiffSingerAcousticFeature DSSP_RunDiffSingerAcousticInferenceTask(DSSP_DiffSingerAcousticInferenceTask task, double duration, DSSP_DiffSingerWords words, DSSP_DiffSingerParameters parameters, DSSP_DiffSingerDynamicMixedSpeakers dynamicMixedSpeakers, float depth, int64_t steps);

// thread-safe
void DSSP_TerminateDiffSingerAcousticInferenceTask(DSSP_DiffSingerAcousticInferenceTask task);

typedef void *DSSP_DiffSingerVocoderInference;

DSSP_DiffSingerVocoderInference DSSP_GetDiffSingerVocoderInference(DSSP_SRTSinger singer);

typedef void *DSSP_DiffSingerVocoderInferenceTask;

DSSP_DiffSingerVocoderInferenceTask DSSP_CreateDiffSingerVocoderInferenceTask(DSSP_DiffSingerVocoderInference inference);
void DSSP_DeleteDiffSingerVocoderInferenceTask(DSSP_DiffSingerVocoderInferenceTask task);
bool DSSP_IsDiffSingerVocoderInferenceTaskError(DSSP_DiffSingerVocoderInferenceTask task);
const char *DSSP_GetDiffSingerVocoderInferenceErrorMessage(DSSP_DiffSingerVocoderInferenceTask task);
DSSP_DiffSingerVocoderInference DSSP_GetDiffSingerVocoderInferenceTaskInference(DSSP_DiffSingerVocoderInferenceTask task);

// Nullable: indicates error
// Reentrant but not thread-safe
DSSP_DiffSingerAudioData DSSP_RunDiffSingerVocoderInferenceTask(DSSP_DiffSingerVocoderInferenceTask task, DSSP_DiffSingerAcousticFeature feature);

// thread-safe
void DSSP_TerminateDiffSingerVocoderInferenceTask(DSSP_DiffSingerVocoderInferenceTask task);

/* ========================================================================
 * dsinfer (audio encoder)
 * ====================================================================== */

DSSP_DiffSingerRawData DSSP_EncodeWAV(DSSP_DiffSingerAudioData audioData);
DSSP_DiffSingerRawData DSSP_EncodeFLAC(DSSP_DiffSingerAudioData audioData);

#ifdef __cplusplus
}
#endif

#endif // DSSP_NATIVE_H
