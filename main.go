package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/iancoleman/orderedmap"
)

type Language struct {
	Code string
	File string
}

var languages = []Language{
	{"en", "locale_en_us.json"},
	{"en", "locale_en_gb.json"},
	{"en", "locale_en_in.json"},
	{"en", "locale_en_au.json"},
	{"es", "locale_es_es.json"},
	{"es", "locale_es_mx.json"},
	{"fr", "locale_fr_fr.json"},
	{"fr", "locale_fr_ca.json"},
	{"ar", "locale_ar_sa.json"},
	{"ar", "locale_ar_eg.json"},
	{"af", "locale_af.json"},
	{"am", "locale_am.json"},
	{"ar", "locale_ar.json"},
	{"az", "locale_az.json"},
	{"be", "locale_be.json"},
	{"bg", "locale_bg.json"},
	{"bn", "locale_bn.json"},
	{"bs", "locale_bs.json"},
	{"ca", "locale_ca.json"},
	{"cs", "locale_cs.json"},
	{"da", "locale_da.json"},
	{"de", "locale_de.json"},
	{"el", "locale_el.json"},
	{"es", "locale_es.json"},
	{"et", "locale_et.json"},
	{"eu", "locale_eu.json"},
	{"fa", "locale_fa.json"},
	{"fi", "locale_fi.json"},
	{"tl", "locale_fil.json"},
	{"fr", "locale_fr.json"},
	{"gl", "locale_gl.json"},
	{"gu", "locale_gu.json"},
	{"he", "locale_he.json"},
	{"hi", "locale_hi.json"},
	{"hr", "locale_hr.json"},
	{"hu", "locale_hu.json"},
	{"hy", "locale_hy.json"},
	{"id", "locale_id.json"},
	{"is", "locale_is.json"},
	{"it", "locale_it.json"},
	{"ja", "locale_ja.json"},
	{"ka", "locale_ka.json"},
	{"kk", "locale_kk.json"},
	{"km", "locale_km.json"},
	{"kn", "locale_kn.json"},
	{"ko", "locale_ko.json"},
	{"ky", "locale_ky.json"},
	{"lo", "locale_lo.json"},
	{"lt", "locale_lt.json"},
	{"lv", "locale_lv.json"},
	{"mk", "locale_mk.json"},
	{"ml", "locale_ml.json"},
	{"mn", "locale_mn.json"},
	{"mr", "locale_mr.json"},
	{"ms", "locale_ms.json"},
	{"my", "locale_my.json"},
	{"no", "locale_nb.json"},
	{"ne", "locale_ne.json"},
	{"nl", "locale_nl.json"},
	{"pa", "locale_pa.json"},
	{"pl", "locale_pl.json"},
	{"ps", "locale_ps.json"},
	{"pt", "locale_pt.json"},
	{"ro", "locale_ro.json"},
	{"ru", "locale_ru.json"},
	{"si", "locale_si.json"},
	{"sk", "locale_sk.json"},
	{"sl", "locale_sl.json"},
	{"sq", "locale_sq.json"},
	{"sr", "locale_sr.json"},
	{"sv", "locale_sv.json"},
	{"sw", "locale_sw.json"},
	{"ta", "locale_ta.json"},
	{"te", "locale_te.json"},
	{"th", "locale_th.json"},
	{"tl", "locale_tl.json"},
	{"tr", "locale_tr.json"},
	{"uk", "locale_uk.json"},
	{"ur", "locale_ur.json"},
	{"uz", "locale_uz.json"},
	{"vi", "locale_vi.json"},
	{"zh", "locale_zh.json"},
	{"zu", "locale_zu.json"},
}

type TranslationSession struct {
	LangCode       string
	FileName       string
	FailedLogFile  string
	TotalStrings   int
	ProcessedCount int
	UnsavedCount   int
	RootTarget     interface{}
}

func main() {
	logFile, err := os.OpenFile("translation.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Fatal: Failed to open log file: %v", err)
	}
	defer logFile.Close()

	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	log.SetFlags(log.Ldate | log.Ltime)

	outputDir := "locale"
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		log.Fatalf("Fatal: Could not create output directory '%s': %v", outputDir, err)
	}

	rawSourceFile := "en.json"
	masterBaseFile := filepath.Join(outputDir, "locale_en.json")

	enData, err := syncMasterEnglish(rawSourceFile, masterBaseFile)
	if err != nil {
		log.Fatalf("Fatal: Master sync failed: %v", err)
	}

	totalKeys := countStrings(enData)
	if totalKeys == 0 {
		log.Printf("⚠️ Found 0 strings in %s. Please add some JSON data to 'en.json' and run again.", masterBaseFile)
		return
	}

	log.Printf("🚀 PHASE 2: Starting parallel translation sync for %d languages...", len(languages))

	const numWorkers = 10
	var wg sync.WaitGroup
	jobs := make(chan Language, len(languages))

	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for lang := range jobs {
				processLanguageFile(lang, enData, totalKeys, outputDir)
			}
		}(w)
	}

	for _, lang := range languages {
		jobs <- lang
	}
	close(jobs)
	wg.Wait()

	log.Printf("🛡️ PHASE 3: Running post-processing placeholder check on all generated JSON files...")
	err = restorePlaceholdersAcrossAllFiles(enData, outputDir)
	if err != nil {
		log.Printf("⚠️ Warning during post-processing placeholder restoration: %v", err)
	}

	log.Printf("🎉 ALL FILES GENERATED, SYNCED, AND POST-PROCESSED SUCCESSFULLY IN '%s' FOLDER!", outputDir)
}

func syncMasterEnglish(rawFile, masterFile string) (interface{}, error) {
	log.Printf("🔄 PHASE 1: Verifying base English files...")

	rawData, err := readOrderedJSON(rawFile)
	if err != nil {
		log.Printf("⚠️ [%s] not found! Creating an empty base file for you...", rawFile)
		emptyData := orderedmap.New()
		err = writeOrderedJSON(rawFile, emptyData)
		if err != nil {
			return nil, err
		}
		rawData = *emptyData
	}

	masterData, err := readOrderedJSON(masterFile)
	if err != nil || masterData == nil {
		log.Printf("📝 [%s] not found. Copying from [%s] for the very first time...", masterFile, rawFile)
		err = writeOrderedJSON(masterFile, rawData)
		return rawData, err
	}

	var added, updated int
	syncedData := mergeEnglishNodes(rawData, masterData, &added, &updated)

	err = writeOrderedJSON(masterFile, syncedData)
	if err != nil {
		return nil, err
	}

	log.Printf("✅ Master file [%s] synchronized: %d added, %d updated.", masterFile, added, updated)
	return syncedData, nil
}

func mergeEnglishNodes(rawNode, masterNode interface{}, added, updated *int) interface{} {
	switch rawVal := rawNode.(type) {
	case string:
		masterStr, ok := masterNode.(string)
		if !ok || masterStr == "" {
			*added++
			return rawVal
		}
		if masterStr != rawVal {
			*updated++
			return rawVal
		}
		return rawVal
	case orderedmap.OrderedMap:
		resultMap := orderedmap.New()
		var masterMap orderedmap.OrderedMap
		if m, ok := masterNode.(orderedmap.OrderedMap); ok {
			masterMap = m
		}
		for _, key := range rawVal.Keys() {
			rawChild, _ := rawVal.Get(key)
			masterChild, _ := masterMap.Get(key)
			resultMap.Set(key, mergeEnglishNodes(rawChild, masterChild, added, updated))
		}
		return *resultMap
	case []interface{}:
		var masterArray []interface{}
		if a, ok := masterNode.([]interface{}); ok {
			masterArray = a
		}
		var resultArray []interface{}
		for i, rawChild := range rawVal {
			var masterChild interface{}
			if i < len(masterArray) {
				masterChild = masterArray[i]
			}
			resultArray = append(resultArray, mergeEnglishNodes(rawChild, masterChild, added, updated))
		}
		return resultArray
	default:
		return rawNode
	}
}

func processLanguageFile(lang Language, enData interface{}, totalKeys int, outputDir string) {
	targetPath := filepath.Join(outputDir, lang.File)
	failedLogPath := filepath.Join(outputDir, fmt.Sprintf("failed_%s.log", lang.Code))
	logPrefix := "[" + targetPath + "]"

	targetData, err := readOrderedJSON(targetPath)
	if err != nil || targetData == nil {
		if _, isMap := enData.(orderedmap.OrderedMap); isMap {
			targetData = *orderedmap.New()
		} else {
			targetData = []interface{}{}
		}
	}

	session := &TranslationSession{
		LangCode:       lang.Code,
		FileName:       targetPath,
		FailedLogFile:  failedLogPath,
		TotalStrings:   totalKeys,
		ProcessedCount: 0,
		UnsavedCount:   0,
		RootTarget:     targetData,
	}

	session.RootTarget = session.processNode(enData, session.RootTarget, logPrefix)
	session.flushToDisk(logPrefix)

	if _, err := os.Stat(failedLogPath); err == nil {
		content, _ := os.ReadFile(failedLogPath)
		if len(strings.TrimSpace(string(content))) == 0 {
			os.Remove(failedLogPath)
		}
	}

	os.Remove(targetPath + ".tmp")
	log.Printf("✅ [COMPLETION ALERT] Synced: %s [%d/%d total strings]", targetPath, session.ProcessedCount, session.TotalStrings)
}

func (s *TranslationSession) processNode(enNode interface{}, targetNode interface{}, logPrefix string) interface{} {
	switch enVal := enNode.(type) {

	case string:
		s.ProcessedCount++

		if targetStr, ok := targetNode.(string); ok && targetStr != "" {
			return targetStr
		}

		var translated string
		if s.LangCode == "en" {
			translated = enVal
		} else {
			var err error
			// MULTI-VARIABLE MASKING PROTECTION PASSED TO TRANSLATOR
			translated, err = translateWithPlaceholderProtection(enVal, "en", s.LangCode, logPrefix)
			if err != nil {
				s.logFailedTranslation(enVal)
				translated = enVal
			} else {
				s.resolveFailedTranslation(enVal)
			}
		}

		s.UnsavedCount++
		log.Printf("%s [%d/%d] [NEW] '%s' -> '%s'", logPrefix, s.ProcessedCount, s.TotalStrings, enVal, translated)

		if s.UnsavedCount >= 20 {
			s.flushToDisk(logPrefix)
		}
		return translated

	case orderedmap.OrderedMap:
		resultMap := orderedmap.New()
		var targetMap orderedmap.OrderedMap
		if t, ok := targetNode.(orderedmap.OrderedMap); ok {
			targetMap = t
		}

		for _, key := range enVal.Keys() {
			enChild, _ := enVal.Get(key)
			targetChild, _ := targetMap.Get(key)
			resultMap.Set(key, s.processNode(enChild, targetChild, logPrefix))
		}
		return *resultMap

	case []interface{}:
		var targetArray []interface{}
		if t, ok := targetNode.([]interface{}); ok {
			targetArray = t
		}

		var resultArray []interface{}
		for i, enChild := range enVal {
			var targetChild interface{}
			if i < len(targetArray) {
				targetChild = targetArray[i]
			}
			resultArray = append(resultArray, s.processNode(enChild, targetChild, logPrefix))
		}
		return resultArray

	default:
		return enNode
	}
}

// Helper: Safely masks multiple variables in exact order before Google touches them
func translateWithPlaceholderProtection(text, from, to, logPrefix string) (string, error) {
	re := regexp.MustCompile(`\$\{[^}]+\}`)
	placeholders := re.FindAllString(text, -1)

	// Step 1: Temporarily mask every ${...} with indexed tokens like __VAR_0__, __VAR_1__
	maskedText := text
	for i, ph := range placeholders {
		token := fmt.Sprintf("__VAR_%d__", i)
		maskedText = strings.Replace(maskedText, ph, token, 1)
	}

	// Step 2: Translate safely via Google API
	translatedMasked, err := translateWithRetry(maskedText, from, to, logPrefix)
	if err != nil {
		return "", err
	}

	// Step 3: Restore the tokens back to their exact original variables
	restoredText := translatedMasked
	for i, ph := range placeholders {
		token := fmt.Sprintf("__VAR_%d__", i)
		restoredText = strings.Replace(restoredText, token, ph, 1)
	}

	// Step 4: Fallback check to make sure no variable was lost during translation
	for _, ph := range placeholders {
		if !strings.Contains(restoredText, ph) {
			restoredText += " " + ph
		}
	}

	return restoredText, nil
}

// Post-processing safety net across all generated JSON files
func restorePlaceholdersAcrossAllFiles(enData interface{}, outputDir string) error {
	files, err := os.ReadDir(outputDir)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`\$\{[^}]+\}`)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") || file.Name() == "locale_en.json" {
			continue
		}

		filePath := filepath.Join(outputDir, file.Name())
		targetData, err := readOrderedJSON(filePath)
		if err != nil {
			continue
		}

		fixedData := fixPlaceholdersNode(enData, targetData, re)
		err = writeOrderedJSON(filePath, fixedData)
		if err != nil {
			log.Printf("⚠️ Failed to write post-processed placeholders for %s: %v", file.Name(), err)
		} else {
			log.Printf("🛡️ Post-processed multi-placeholder check passed for: %s", file.Name())
		}
	}
	return nil
}

func fixPlaceholdersNode(enNode interface{}, targetNode interface{}, re *regexp.Regexp) interface{} {
	switch enVal := enNode.(type) {
	case string:
		targetStr, ok := targetNode.(string)
		if !ok {
			return enNode
		}

		enPlaceholders := re.FindAllString(enVal, -1)
		if len(enPlaceholders) == 0 {
			return targetStr
		}

		fixedStr := targetStr
		targetPlaceholders := re.FindAllString(targetStr, -1)

		// Substitute or append multiple placeholders sequentially by index order
		for i, originalPh := range enPlaceholders {
			if i < len(targetPlaceholders) {
				fixedStr = strings.Replace(fixedStr, targetPlaceholders[i], originalPh, 1)
			} else {
				fixedStr = strings.TrimSpace(fixedStr) + " " + originalPh
			}
		}
		return fixedStr

	case orderedmap.OrderedMap:
		resultMap := orderedmap.New()
		var targetMap orderedmap.OrderedMap
		if t, ok := targetNode.(orderedmap.OrderedMap); ok {
			targetMap = t
		}

		for _, key := range enVal.Keys() {
			enChild, _ := enVal.Get(key)
			targetChild, _ := targetMap.Get(key)
			resultMap.Set(key, fixPlaceholdersNode(enChild, targetChild, re))
		}
		return *resultMap

	case []interface{}:
		var targetArray []interface{}
		if t, ok := targetNode.([]interface{}); ok {
			targetArray = t
		}

		var resultArray []interface{}
		for i, enChild := range enVal {
			var targetChild interface{}
			if i < len(targetArray) {
				targetChild = targetArray[i]
			}
			resultArray = append(resultArray, fixPlaceholdersNode(enChild, targetChild, re))
		}
		return resultArray

	default:
		return targetNode
	}
}

func (s *TranslationSession) logFailedTranslation(text string) {
	f, err := os.OpenFile(s.FailedLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(text + "\n")
}

func (s *TranslationSession) resolveFailedTranslation(text string) {
	bytes, err := os.ReadFile(s.FailedLogFile)
	if err != nil {
		return
	}

	lines := strings.Split(string(bytes), "\n")
	var remainingLines []string
	found := false

	for _, line := range lines {
		if line == text {
			found = true
			continue
		}
		if line != "" {
			remainingLines = append(remainingLines, line)
		}
	}

	if found {
		os.WriteFile(s.FailedLogFile, []byte(strings.Join(remainingLines, "\n")+"\n"), 0644)
	}
}

func (s *TranslationSession) flushToDisk(logPrefix string) {
	err := writeOrderedJSON(s.FileName, s.RootTarget)
	if err != nil {
		log.Printf("%s ⚠ Warning: Failed to save: %v", logPrefix, err)
		return
	}
	if s.UnsavedCount > 0 {
		log.Printf("%s 💾 [CHECKPOINT] Synchronized %d new records safely to disk", logPrefix, s.UnsavedCount)
		s.UnsavedCount = 0
	}
}

func translateTextNative(text, from, to string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return text, nil
	}

	urlStr := fmt.Sprintf(
		"https://translate.googleapis.com/translate_a/single?client=gtx&sl=%s&tl=%s&dt=t&q=%s",
		from, to, url.QueryEscape(text),
	)

	resp, err := http.Get(urlStr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var data []interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	if len(data) > 0 {
		if innerData, ok := data[0].([]interface{}); ok {
			var translated string
			for _, slice := range innerData {
				if parts, ok := slice.([]interface{}); ok && len(parts) > 0 {
					if str, ok := parts[0].(string); ok {
						translated += str
					}
				}
			}
			return translated, nil
		}
	}

	return "", fmt.Errorf("unexpected response format")
}

func translateWithRetry(text, from, to, logPrefix string) (string, error) {
	maxRetries := 5
	backoff := 3 * time.Second

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		translated, err := translateTextNative(text, from, to)
		if err == nil {
			time.Sleep(1500 * time.Millisecond)
			return translated, nil
		}

		lastErr = err
		log.Printf("%s ⚠ Translation failed (Attempt %d/%d). Retrying in %v...", logPrefix, i+1, maxRetries, backoff)
		time.Sleep(backoff)
		backoff *= 2
	}

	log.Printf("%s ❌ Permanently failed after %d attempts for text: '%s'", logPrefix, maxRetries, text)
	return "", lastErr
}

func countStrings(node interface{}) int {
	count := 0
	switch val := node.(type) {
	case string:
		count++
	case orderedmap.OrderedMap:
		for _, key := range val.Keys() {
			child, _ := val.Get(key)
			count += countStrings(child)
		}
	case []interface{}:
		for _, child := range val {
			count += countStrings(child)
		}
	}
	return count
}

func readOrderedJSON(filename string) (interface{}, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		tmpFilename := filename + ".tmp"
		if tmpBytes, tmpErr := os.ReadFile(tmpFilename); tmpErr == nil {
			log.Printf("🔄 [RECOVERED] Found interrupted save file: %s. Retracing...", tmpFilename)
			bytes = tmpBytes
			err = nil
		} else {
			return nil, err
		}
	}

	o := orderedmap.New()
	if jsonErr := json.Unmarshal(bytes, o); jsonErr == nil {
		return *o, nil
	}

	var a []interface{}
	if jsonErr := json.Unmarshal(bytes, &a); jsonErr == nil {
		return a, nil
	}

	tmpFilename := filename + ".tmp"
	if tmpBytes, tmpErr := os.ReadFile(tmpFilename); tmpErr == nil {
		log.Printf("🔄 [RECOVERED] Main file corrupted. Retracing from backup %s...", tmpFilename)
		if jsonErr := json.Unmarshal(tmpBytes, o); jsonErr == nil {
			return *o, nil
		}
		if jsonErr := json.Unmarshal(tmpBytes, &a); jsonErr == nil {
			return a, nil
		}
	}

	return nil, fmt.Errorf("invalid JSON format")
}

func writeOrderedJSON(filename string, data interface{}) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmpFilename := filename + ".tmp"
	err = os.WriteFile(tmpFilename, bytes, 0644)
	if err != nil {
		return err
	}
	return os.Rename(tmpFilename, filename)
}
