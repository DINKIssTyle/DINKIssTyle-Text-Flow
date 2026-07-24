import Foundation

#if canImport(FoundationModels)
import FoundationModels
#endif

private struct HelperRequest: Decodable {
    let mode: String
    let instructions: String?
    let prompt: String?
}

private struct HelperResponse: Encodable {
    let available: Bool
    let state: String
    let content: String?
    let error: String?

    init(available: Bool, state: String, content: String? = nil, error: String? = nil) {
        self.available = available
        self.state = state
        self.content = content
        self.error = error
    }
}

@main
private struct AppleIntelligenceHelper {
    static func main() async {
        do {
            let input = FileHandle.standardInput.readDataToEndOfFile()
            let request = try JSONDecoder().decode(HelperRequest.self, from: input)
            let response = await handle(request)
            try write(response)
        } catch {
            try? write(
                HelperResponse(
                    available: false,
                    state: "unavailable",
                    error: error.localizedDescription
                )
            )
        }
    }

    private static func handle(_ request: HelperRequest) async -> HelperResponse {
        #if canImport(FoundationModels)
        guard #available(macOS 26.0, *) else {
            return HelperResponse(
                available: false,
                state: "os_unsupported",
                error: "Apple Intelligence requires macOS 26 or later."
            )
        }

        switch SystemLanguageModel.default.availability {
        case .available:
            if request.mode == "availability" {
                return HelperResponse(available: true, state: "available")
            }
        case .unavailable(let reason):
            switch reason {
            case .deviceNotEligible:
                return HelperResponse(
                    available: false,
                    state: "device_not_eligible",
                    error: "This Mac does not support Apple Intelligence."
                )
            case .appleIntelligenceNotEnabled:
                return HelperResponse(
                    available: false,
                    state: "not_enabled",
                    error: "Apple Intelligence is not enabled in System Settings."
                )
            case .modelNotReady:
                return HelperResponse(
                    available: false,
                    state: "model_not_ready",
                    error: "The Apple Intelligence model is not ready."
                )
            @unknown default:
                return HelperResponse(
                    available: false,
                    state: "unavailable",
                    error: "Apple Intelligence is unavailable."
                )
            }
        }

        guard request.mode == "generate" else {
            return HelperResponse(
                available: false,
                state: "unavailable",
                error: "Unsupported helper mode."
            )
        }
        let instructions = request.instructions?
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        let prompt = request.prompt?
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !prompt.isEmpty else {
            return HelperResponse(
                available: true,
                state: "available",
                error: "The prompt is empty."
            )
        }

        do {
            let session = LanguageModelSession(instructions: instructions)
            let result = try await session.respond(to: prompt)
            let content = result.content.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !content.isEmpty else {
                return HelperResponse(
                    available: true,
                    state: "available",
                    error: "Apple Intelligence returned an empty response."
                )
            }
            return HelperResponse(available: true, state: "available", content: content)
        } catch {
            return HelperResponse(
                available: true,
                state: "available",
                error: error.localizedDescription
            )
        }
        #else
        return HelperResponse(
            available: false,
            state: "sdk_unavailable",
            error: "This build does not include the Foundation Models framework."
        )
        #endif
    }

    private static func write(_ response: HelperResponse) throws {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.sortedKeys]
        let data = try encoder.encode(response)
        FileHandle.standardOutput.write(data)
    }
}
