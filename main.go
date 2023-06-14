package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/Bureau-Inc/overwatch-common/logger"
	"github.com/Bureau-Inc/overwatch-common/logger/tag"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rekognition"
)

const (
	panDoc     = "PAN"
	aadharDoc  = "AADHAR"
	unknownDoc = "UNKNOWN"
)

type payload struct {
	DocType      string `json:"docType"`
	CountryCode  string `json:"countryCode"`
	FrontURL     string `json:"frontUrl"`
	BackURL      string `json:"backUrl"`
	GPTPrompt    string `json:"prompt"`
	OutputFields string `json:"outputFields"`
}

func buildErrorResp(err error) events.APIGatewayProxyResponse {
	e := map[string]interface{}{
		"error": err.Error(),
	}

	b, _ := json.Marshal(e)

	return events.APIGatewayProxyResponse{
		StatusCode: 500,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body:            string(b),
		IsBase64Encoded: false,
	}
}

func buildSuccessResponse(d map[string]interface{}) events.APIGatewayProxyResponse {
	b, _ := json.Marshal(d)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body:            string(b),
		IsBase64Encoded: false,
	}
}

func HandleRequest(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	input := &payload{}
	if err := json.Unmarshal([]byte(req.Body), input); err != nil {
		return buildErrorResp(fmt.Errorf("failed to unmarshal input:%v", err)), nil
	}

	logger.INFO("got input", tag.NewAnyTag("input", input))

	var result map[string]interface{}
	var err error
	var docType string
	if input.DocType == "" {
		docType = "UNKNOWN"
	} else {
		docType = strings.ToUpper(input.DocType)
	}
	switch docType {
	case panDoc:
		result, err = doDocAnalysis(input.FrontURL, panDoc, "")
		if err != nil {
			logger.ERROR("failed to do pan analysis", tag.NewErrorTag(err))
			return buildErrorResp(fmt.Errorf("failed to do pan analysis: %v", err)), nil
		}
	case aadharDoc:
		result, err = doDocAnalysis(input.FrontURL, aadharDoc, "")
		if err != nil {
			logger.ERROR("failed to do aadhar analysis", tag.NewErrorTag(err))
			return buildErrorResp(fmt.Errorf("failed to do aadhar analysis: %v", err)), nil
		}
	case unknownDoc:
		// if outputJSON is absent, return error
		if input.OutputFields == "" {
			return buildErrorResp(fmt.Errorf("for empty docType provide a desired output JSON in OutputJSON")), nil
		} else {
			result, err = doDocAnalysis(input.FrontURL, unknownDoc, input.OutputFields)
			if err != nil {
				logger.ERROR("failed to do unknown doc analysis", tag.NewErrorTag(err))
				return buildErrorResp(fmt.Errorf("failed to do unknown doc analysis: %v", err)), nil
			}
		}

	}

	logger.INFO("got output", tag.NewAnyTag("output", result))
	return buildSuccessResponse(result), nil
}

// 		"body": "{\"docType\":\"pan\",\"frontUrl\":\"https://i.ibb.co/jHpMgnw/a597c560-ce50-47e5-bf73-ebe7418d1200.jpg\"}",
// 		"body": "{\"frontUrl\":\"https://i.ibb.co/jHpMgnw/a597c560-ce50-47e5-bf73-ebe7418d1200.jpg\"\"outputFields\": \"{[\"name\"]}",

func main() {
	// read from event.json file
	event := []byte(`{
		"type": "REQUEST",
		"methodArn": "arn:aws:execute-api:us-east-1:123456789012:abcdef123/test/GET/request",
		"resource": "/request",
		"body": "{\"docType\":\"\",\"frontUrl\":\"https://officeanywhere.io/images/payslip.jpg\", \"outputFields\": \"name\"}",
		"path": "/request",
		"httpMethod": "POST",
		"headers": {
		  "X-AMZ-Date": "20170718T062915Z",
		  "Accept": "*/*",
		  "HeaderAuth1": "headerValue1",
		  "CloudFront-Viewer-Country": "US",
		  "CloudFront-Forwarded-Proto": "https",
		  "CloudFront-Is-Tablet-Viewer": "false",
		  "CloudFront-Is-Mobile-Viewer": "false",
		  "User-Agent": "..."
		},
		"queryStringParameters": {
		  "QueryString1": "queryValue1"
		},
		"pathParameters": {},
		"stageVariables": {
		  "StageVar1": "stageValue1"
		},
		"requestContext": {
		  "path": "/request",
		  "accountId": "123456789012",
		  "resourceId": "05c7jb",
		  "stage": "test",
		  "requestId": "...",
		  "identity": {
			"apiKey": "...",
			"sourceIp": "...",
			"clientCert": {
			  "clientCertPem": "CERT_CONTENT",
			  "subjectDN": "www.example.com",
			  "issuerDN": "Example issuer",
			  "serialNumber": "a1:a1:a1:a1:a1:a1:a1:a1:a1:a1:a1:a1:a1:a1:a1:a1",
			  "validity": {
				"notBefore": "May 28 12:30:02 2019 GMT",
				"notAfter": "Aug  5 09:36:04 2021 GMT"
			  }
			}
		  },
		  "resourcePath": "/request",
		  "httpMethod": "GET",
		  "apiId": "abcdef123"
		}
	}`)

	// unmarshal events.APIGatewayProxyRequest
	var req events.APIGatewayProxyRequest
	if err := json.Unmarshal(event, &req); err != nil {
		fmt.Println(err)
	}

	lambda.Start(HandleRequest)
	// HandleRequest(req)
}

func doDocAnalysis(imageURL string, docType string, desiredFields string) (map[string]interface{}, error) {
	// azure ocr analysis
	analysisReqID, err := submitOCRAnalysis(imageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to submit ocr analysis: %v", err)
	}

	time.Sleep(2 * time.Second)

	result, err := fetchOCRAnalysisResult(analysisReqID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ocr analysis result: %v", err)
	}
	azureOCRContent := result.AnalyzeResult.Content

	// aws ocr analysis
	resultfromaws, err := fetchOCRAnalysisResultfromAWS(imageURL)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch ocr analysis result from aws: %v", err)
	}

	awsOCRContent := fetchFullTextFromOCRTextAWS(resultfromaws)

	logger.INFO("got result from aws", tag.NewAnyTag("result", awsOCRContent))
	logger.INFO("got result from azure", tag.NewAnyTag("result", azureOCRContent))

	azureResultScore := fetchCombinedNormalizedConfidenceScoreForAzure(&result.AnalyzeResult)
	logger.INFO("got result from azure", tag.NewAnyTag("result", azureResultScore))

	awsResultScore := fetchCombinedNormalizedConfidenceScoreForAWS(resultfromaws)
	logger.INFO("got result from aws", tag.NewAnyTag("result", awsResultScore))

	gptResultRawAzure, err := askGPTForPIIAnalysis(azureOCRContent, docType, desiredFields)
	if err != nil {
		return nil, fmt.Errorf("failed to ask gpt for PII analysis: %v", err)
	}

	gptResultRawAWS, err := askGPTForPIIAnalysis(awsOCRContent, docType, desiredFields)
	if err != nil {
		return nil, fmt.Errorf("failed to ask gpt for PII analysis: %v", err)
	}

	gptResultAzureMap := map[string]interface{}{}
	err = json.Unmarshal([]byte(gptResultRawAzure), &gptResultAzureMap)
	if err != nil {
		return nil, fmt.Errorf("failed to decode gpt result: %v", err)
	}

	gptResultAWSMap := map[string]interface{}{}
	err = json.Unmarshal([]byte(gptResultRawAWS), &gptResultAWSMap)
	if err != nil {
		return nil, fmt.Errorf("failed to decode gpt result: %v", err)
	}

	// return the map that has more number of non null values. If both have same number of non null values then return the map with higher confidence score
	azureNonNullCount := 0
	awsNonNullCount := 0
	for _, v := range gptResultAzureMap {
		logger.INFO("azure result", tag.NewAnyTag("result", v))
		if v != "nil" {
			azureNonNullCount++
		}
	}

	for _, v := range gptResultAWSMap {
		if v != "nil" {
			awsNonNullCount++
		}
	}

	logger.INFO("azure non null count", tag.NewAnyTag("count", azureNonNullCount))
	logger.INFO("aws non null count", tag.NewAnyTag("count", awsNonNullCount))

	if azureNonNullCount > awsNonNullCount {
		logger.INFO("azure non null count is higher")
		return gptResultAzureMap, nil
	} else if azureNonNullCount < awsNonNullCount {
		logger.INFO("aws non null count is higher")
		return gptResultAWSMap, nil
	} else {
		if azureResultScore > awsResultScore {
			logger.INFO("azure result score is higher")
			return gptResultAzureMap, nil
		}
		logger.INFO("aws result score is higher")
		return gptResultAWSMap, nil
	}
}

func submitOCRAnalysis(imageURL string) (requestID string, err error) {
	ocrServiceHostURL := "https://idv-ocr-poc.cognitiveservices.azure.com/formrecognizer/documentModels/prebuilt-read:analyze?api-version=2022-08-31&stringIndexType=textElements"

	payload := strings.NewReader(fmt.Sprintf(`{"urlSource": "%s"}`, imageURL))
	req, err := http.NewRequest("POST", ocrServiceHostURL, payload)
	if err != nil {
		return "", fmt.Errorf("failed to construct request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// TODO: get the key from env
	req.Header.Set("Ocp-Apim-Subscription-Key", "cebb95ebad534bdba340eed6556691d2")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	logger.INFO("received status code for submitOCRAnalysis", tag.NewAnyTag("httpCode", resp.StatusCode))

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	logger.INFO("received response for askGPTForPIIAnalysis", tag.NewStringTag("response", string(respBytes)))

	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("received unexpected status code in response: %d", resp.StatusCode)
	}

	return resp.Header.Get("apim-request-id"), nil
}

func fetchOCRAnalysisResultfromAWS(imageURL string) (*rekognition.DetectTextOutput, error) {
	// upload image to s3
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("ap-south-1"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	// image URL to Bytes array
	resp, err := http.Get(imageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get image from URL: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image from response: %v", err)
	}

	// call AWS rekognition
	svc2 := rekognition.New(sess)
	imageResp, err := svc2.DetectText(&rekognition.DetectTextInput{
		Image: &rekognition.Image{
			Bytes: body,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call AWS rekognition: %v", err)
	}
	return imageResp, nil
}

func fetchFullTextFromOCRTextAWS(imageResp *rekognition.DetectTextOutput) string {
	// append all the response text into a single string
	var sb strings.Builder
	for _, text := range imageResp.TextDetections {
		// check for LINE type
		if *text.Type == "WORD" {
			sb.WriteString(*text.DetectedText)
			sb.WriteString(" ")
		}
	}
	return sb.String()
}

func fetchOCRAnalysisResult(requestID string) (*OCRAnalysisResult, error) {
	hostURL := fmt.Sprintf("https://idv-ocr-poc.cognitiveservices.azure.com/formrecognizer/documentModels/prebuilt-read/analyzeResults/%s?api-version=2022-08-31", requestID)

	req, err := http.NewRequest("GET", hostURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request: %v", err)
	}

	// TODO: get the key from env
	req.Header.Set("Ocp-Apim-Subscription-Key", "cebb95ebad534bdba340eed6556691d2")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	logger.INFO("received status code for fetchOCRAnalysisResult", tag.NewAnyTag("httpCode", resp.StatusCode))

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// logger.INFO("received response for fetchOCRAnalysisResult", tag.NewStringTag("response", string(respBytes)))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received unexpected status code in response: %d", resp.StatusCode)
	}

	d := &OCRAnalysisResult{}
	err = json.Unmarshal(respBytes, d)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body: %v", err)
	}

	return d, nil
}

func fetchCombinedNormalizedConfidenceScoreForAzure(result *AnalyzeResult) float64 {
	confidenceScore := 0.0
	countWords := 0
	for _, p := range result.Pages {
		for _, w := range p.Words {
			confidenceScore += w.Confidence
			countWords++
		}
	}
	// normalize the confidence score 0-10 and round it to int value
	confidenceScore = confidenceScore / float64(countWords) * 10
	confidenceScore = math.Round(confidenceScore)
	return confidenceScore
}

func fetchCombinedNormalizedConfidenceScoreForAWS(input *rekognition.DetectTextOutput) float64 {
	confidenceScore := 0.0
	countWords := 0
	for _, text := range input.TextDetections {
		// check for LINE type
		if *text.Type == "WORD" {
			confidenceScore += *text.Confidence / 10
			countWords++
		}
	}
	// normalize the confidence score 0-10 from 0-100 and round it to int value
	confidenceScore = confidenceScore / float64(countWords)
	confidenceScore = math.Round(confidenceScore)
	return confidenceScore
}

func askGPTForPIIAnalysis(ocrExtractedText string, docType string, desiredJSON string) (string, error) {
	escaped := strings.ReplaceAll(ocrExtractedText, "\n", "\\n")

	var prompt string
	switch docType {
	case aadharDoc:
		prompt = systemPrompt + aadharPrompt + escaped + endPrompt
	case panDoc:
		prompt = systemPrompt + panPrompt + escaped + endPrompt
		print("pan prompt: ", prompt)
	case unknownDoc:

		// escapedDesiredJson := strings.ReplaceAll(string(desiredJSON), "\n", "\\n")
		prompt = systemPrompt + noneTypeDocPrompt + escaped + desiredJsonPrefixPrompt + desiredJSON + endPrompt
	}
	logger.INFO(prompt)

	hostURL := "https://bureauteam1.openai.azure.com/openai/deployments/gpt-turbo/chat/completions?api-version=2023-05-15"

	logger.INFO("sending request to askGPTForPIIAnalysis", tag.NewStringTag("request", prompt))

	req, err := http.NewRequest("POST", hostURL, strings.NewReader(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to construct request: %v", err)
	}

	// TODO: get the key from env
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("api-key", "2f70fa159ee04c81b94624e3cbdb41b4")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	logger.INFO("received status code for askGPTForPIIAnalysis", tag.NewAnyTag("httpCode", resp.StatusCode))

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	logger.INFO("received response for askGPTForPIIAnalysis", tag.NewStringTag("response", string(respBytes)))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received unexpected status code in response: %d", resp.StatusCode)
	}

	d := &GPTResult{}
	err = json.Unmarshal(respBytes, d)
	if err != nil {
		return "", fmt.Errorf("failed to decode response body: %v", err)
	}

	return d.Choices[0].Message.Content, nil
}

var systemPrompt = `{
"messages": [
{
	"role": "system",
	"content": "You are a helpful assistant."
},
`
var aadharPrompt = `
	{
		"role": "user",
		"content": "I want you to act as OCR and PII Expert.\n\nI will provide you an ocr extracted sample text from an specific document type \"aadhaar\" tells you about different PII types in text. You need indentify those PII classes.\n\nDoc Type: \"aadhaar\"\nSample OCR extracted text:\n भारत सरकार Government of India\nA\nकुणाल गौरव Kudal Gaurav जन्म तिथि/DOB: 05/12/1989 SOU/ MALE\nS\nS\nK\n5245 5210 8456\nमेरा आधार, मेरी पहचान\nIn the above text:\n\nKudal Gaurav is Name type PII\n05/12/1989 is Dob type PII\n524552108456 is Aadhaar number type PII\nMale is the Gender PII\n\nUnderstand the positions of different PII types in the extracted text. In the next prompts I will provide you with sample ocr text in json format and you need to answer the following fields:\n\n- Name\n- Date of Birth\n- Aadhaar number\nGender\n\nThe output json looks like below.\n\n{\"fullName\": {$Name}, \"dateOfBirth\": {$Date of Birth}, \"docNumber\": {$Aadhaar number}, \"geder\": {$Gender}, \"docType\": {$Doc Type}}\n\n\nI am describing the Input format:\n\n[Input]\nDocType: \"some text\"\nOCR Text: \"some text\. Send a nil value if the field is not present in the text.""
	},
	{
		"role": "user",
		"content": "[no prose]\n[output only in json and return all keys with nil value in case you can't find that value]\nDocType: \"aadhaar\"\nOCR Text: \"\n भारत सरकार Government of India\nA\nकुणाल गौरव Kudal Gaurav जन्म तिथि/DOB: 05/12/1989 SOU/ MALE\nS\nS\nK\n5245 5210 8456\nमेरा आधार, मेरी पहचान"
	},
	{
		"role": "assistant",
		"content": "{\n    \"fullName\": \"Kudal Gaurav\",\n    \"dateOfBirth\": \"05/12/1989\",\n    \"docNumber\": \"524552108456\",\n    \"docType\": \"aadhaar\"\n}"
	},
	{
		"role": "user",
		"content": "[no prose]\n[output only in json and return all keys with nil value in case you can't find that value]\nDocType: \"aadhaar\"\nOCR Text: \"
	
`

var panPrompt = `
{
	"role": "user",
	"content": "I want you to act as OCR and PII Expert.\n\nI will provide you an ocr extracted sample text from an specific document type \"PAN\" tells you about different PII types in text. You need indentify those PII classes.\n\nDoc Type: \"PAN\"\nSample OCR extracted text:\n\nआयकर विभाग INCOME TAX DEPARTMENT SHEKH ATAUL\\nSHEKH MUJAFFAR ALI\\n03/02/1998 Permanent Account Number BWPPA3202G\\nSignature\\nभारत सरकार GOVT. OF INDIA\\n16082016\n\n\nIn the above text:\n\nSHEKH ATAUL is Name type PII\nSHEKH MUJAFFAR ALI is Fathers Name type PII\n03/02/1998 is Dob type PII\nBWPPA3202G is Pan number type PII\n16/08/2016 is Issue date type PII\n\n\nUnderstand the positions of different PII types in the extracted text. In the next prompts I will provide you with sample ocr text in json format and you need to answer the following fields:\n\n- Name\n- Fathers Name\n- Date of Birth\n- Pan number\n- Issuer Date\n\n\nThe output json looks like below.\n\n{\"fullName\": {$Name}, \"fatherName\": {$Fathers Name}, \"dateOfBirth\": {$Date of Birth}, \"docNumber\": {$Pan number}, \"issueDate\": {$Issuer Date}, \"docType\": {$Doc Type}}\n\n\nI am describing the Input format:\n\n[Input]\nDocType: \"some text\"\nOCR Text: \"some text\""
},
{
	"role": "user",
	"content": "[no prose]\n[output only in json]\nDocType: \"PAN\"\nOCR Text: \"भारत सरकार\\nआयकर विभाग\\nINCOME TAX DEPARTMENT\\nSANTOSHBHAI BHAVSAR\\nSUKHLAL JAGANNATH BHAVSAR\\n02/07/1976\\nPermanent Account Number APWPB3057M\\nS.S land\\nSignature\\nGOVT. OF INDIA\\n10052008\"2\""
},
{
	"role": "assistant",
	"content": "{\n    \"fullName\": \"SANTOSHBHAI BHAVSAR\",\n    \"fatherName\": \"SUKHLAL JAGANNATH BHAVSAR\",\n    \"dateOfBirth\": \"02/07/1976\",\n    \"docNumber\": \"APWPB3057M\",\n    \"issueDate\": \"10/05/2008\",\n    \"docType\": \"PAN\"\n}"
},
{
	"role": "user",
	"content": "[no prose]\n[output only in json and return nil when there are no values ]\nDocType: \"PAN\"\nOCR Text: \"
`

var noneTypeDocPrompt = `
{
	"role": "user",
	"content": "I want you to act as an OCR and PII Expert.\n\nI will provide you with a sample text extracted from an unspecified document. The text contains information about different types of Personally Identifiable Information (PII). Your task is to identify these PII classes. In the next prompt, I will provide you with a list of fields that I need to extract from the OCR text. You should return an output JSON that contains exactly these keys from the desired fields, along with their corresponding values found in the OCR text. If a field is not present in the text, please provide a nil value for that field."
	},
	{
	"role": "user",
	"content": "[no prose]\n[Output should be in the desired JSON format, including all keys with a nil value if the value couldn't be found in the OCR text.]OCR Text: "
`
var desiredJsonPrefixPrompt = ` and Desired fields:`

var endPrompt = `,
}
]
}`

// {
//     "messages": [
//         {
//             "role": "system",
//             "content": "You are a helpful assistant."
//         },
//         {
//             "role": "user",
//             "content": "I want you to act as OCR and PII Expert.\n\nI will provide you an ocr extracted sample text from an specific document type \"PAN\" tells you about different PII types in text. You need indentify those PII classes.\n\nDoc Type: \"PAN\"\nSample OCR extracted text:\n\nआयकर विभाग INCOME TAX DEPARTMENT SHEKH ATAUL\\nSHEKH MUJAFFAR ALI\\n03/02/1998 Permanent Account Number BWPPA3202G\\nSignature\\nभारत सरकार GOVT. OF INDIA\\n16082016\n\n\nIn the above text:\n\nSHEKH ATAUL is Name type PII\nSHEKH MUJAFFAR ALI is Fathers Name type PII\n03/02/1998 is Dob type PII\nBWPPA3202G is Pan number type PII\n16/08/2016 is Issue date type PII\n\n\nUnderstand the positions of different PII types in the extracted text. In the next prompts I will provide you with sample ocr text in json format and you need to answer the following fields:\n\n- Name\n- Fathers Name\n- Date of Birth\n- Pan number\n- Issuer Date\n\n\nThe output json looks like below.\n\n{\"fullName\": {$Name}, \"fatherName\": {$Fathers Name}, \"dateOfBirth\": {$Date of Birth}, \"docNumber\": {$Pan number}, \"issueDate\": ${Issuer Date}, \"docType\": {$Doc Type}}\n\n\nI am describing the Input format:\n\n[Input]\nDocType: \"some text\"\nOCR Text: \"some text\""
//         },
//         {
//             "role": "user",
//             "content": "[no prose]\n[output only in json]\nDocType: \"PAN\"\nOCR Text: \"भारत सरकार\\nआयकर विभाग\\nINCOME TAX DEPARTMENT\\nSANTOSHBHAI BHAVSAR\\nSUKHLAL JAGANNATH BHAVSAR\\n02/07/1976\\nPermanent Account Number APWPB3057M\\nS.S land\\nSignature\\nGOVT. OF INDIA\\n10052008\"2\""
//         },
//         {
//             "role": "assistant",
//             "content": "{\n    \"fullName\": \"SANTOSHBHAI BHAVSAR\",\n    \"fatherName\": \"SUKHLAL JAGANNATH BHAVSAR\",\n    \"dateOfBirth\": \"02/07/1976\",\n    \"docNumber\": \"APWPB3057M\",\n    \"issueDate\": \"10/05/2008\",\n    \"docType\": \"PAN\"\n}"
//         },
//         {
//             "role": "user",
//             "content": "[no prose]\n[output only in json]\nDocType: \"PAN\"\nOCR Text: \"\""
//         }
//     ]
// }
