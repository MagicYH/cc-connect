
export const meta = {
  name: 'feishu-mention-slash-cmd',
  description: 'Investigate why slash commands fail when Bot1 mentions Bot2 in Feishu messages',
  phases: [
    { title: 'Understand', detail: 'Trace Feishu message handling, mention parsing, and slash command flow' },
    { title: 'Verify', detail: 'Cross-verify findings across message reception, command extraction, and routing' },
    { title: 'Synthesize', detail: 'Root cause analysis and fix recommendation' },
  ],
}

phase('Understand')

// 1. Understand the Feishu message reception flow
const feishuFlow = await agent(
  `Read the Feishu platform implementation in platform/feishu/ directory. I need to understand:
1. How incoming messages are received (event handler/callback)
2. How mentions (@bot) are detected and parsed from message content
3. How slash commands are detected and parsed
4. How the message text is cleaned/stripped before being sent to the engine
5. How the message is routed (which bot handles it when Bot1 mentions Bot2)
Focus on the message reception path from HTTP callback to engine.HandleMessage. Report all relevant code paths with file paths and line numbers.`,
  { label: 'feishu-flow', phase: 'Understand' }
)

// 2. Understand the core engine message handling
const engineHandling = await agent(
  `Read the core engine implementation in core/engine.go and related files. I need to understand:
1. How HandleMessage processes incoming messages
2. How slash commands are detected and routed (look for IsCommand, CommandPrefix, "/" handling)
3. How mentions interact with command detection
4. The message text normalization/cleaning pipeline
5. How the engine decides which agent session to route a message to
Report all relevant code with file paths and line numbers.`,
  { label: 'engine-handling', phase: 'Understand' }
)

// 3. Understand how Feishu message content is structured (with mentions)
const messageContent = await agent(
  `In the Feishu/Lark platform code (platform/feishu/), investigate:
1. How the raw message event JSON looks - what fields contain mention info
2. How message.Text is constructed when there are @mentions (mention objects, at_user_id, etc.)
3. How mention text is stripped/cleaned from the message content
4. Whether there's special handling for messages that contain BOTH a mention AND a slash command (e.g., "@Bot2 /help")
5. Look for any mention of "at_user_id", "mention", "AtUser" in the Feishu platform code
Also check core/ for any mention-related or command-prefix processing logic.
Report exact code with file paths and line numbers.`,
  { label: 'message-content', phase: 'Understand' }
)

phase('Verify')

const verification = await agent(
  `Based on the following investigation context, I need you to verify a specific hypothesis about why slash commands fail when Bot1 mentions Bot2 in Feishu:

The hypothesis: When Bot1 sends a message like "@Bot2 /help" in a Feishu group, the mention (@Bot2) is either:
A) Not properly stripped from the message text, so the engine sees something like "@_user_1 /help" instead of "/help"
B) The mention stripping removes or corrupts the slash command prefix
C) The message routing logic sees the mention but doesn't recognize it as a command because the slash isn't at the start of the text
D) The message is routed to Bot1's session instead of Bot2's session because the sender is Bot1

Your tasks:
1. Read platform/feishu/feishu.go - focus on how message text is extracted and cleaned
2. Read core/engine.go - focus on command detection (look for strings.HasPrefix, "/", "CommandPrefix", IsCommand)
3. Read core/message.go or similar - focus on the Message struct and how text flows
4. Check if there's a TextWithoutMention or CleanText field
5. Look for any mention of "at_user" or "mention" in the text cleaning pipeline
6. Trace the exact sequence: Feishu event → text extraction → mention stripping → command detection → routing

Report the EXACT root cause with file paths and line numbers.`,
  { label: 'verify-root-cause', phase: 'Verify' }
)

phase('Synthesize')

const synthesis = await agent(
  `You are a senior Go engineer. Based on the investigation of cc-connect's Feishu integration, synthesize a root cause analysis.

Context: The user reports that when Bot1 mentions Bot2 in a Feishu message with a slash command (e.g., "@Bot2 /help"), Bot2 cannot process the slash command.

The key code paths to consider:
1. Feishu message reception: platform/feishu/ - how the raw event is parsed and text extracted
2. Mention handling: How @mentions are detected and stripped from message text
3. Command detection: core/engine.go - how "/" prefix is checked to identify commands
4. Message routing: How the engine decides which session handles the message

Common failure modes in messaging bots:
- Mention text left in the message content causes "/" to not be at position 0
- The mention stripping uses a placeholder like "@_user_1" that shifts the slash position
- The slash command check uses the raw text before mention stripping
- The message is routed based on sender rather than mentionee
- The Feishu API returns mention info separately from text, but text still contains @mention placeholder

Please read the actual code files to verify your analysis:
1. platform/feishu/feishu.go (or whatever the main file is)
2. core/engine.go
3. Any i18n or command-related files

Provide:
1. The EXACT root cause (with code references)
2. The message text at each stage of processing (raw → after mention strip → at command check)
3. A concrete fix recommendation`,
  { label: 'synthesize', phase: 'Synthesize' }
)

return { feishuFlow, engineHandling, messageContent, verification, synthesis }
