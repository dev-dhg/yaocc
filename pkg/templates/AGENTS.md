# AGENTS.md - Your Workspace

This folder is home. Treat it that way.

## First Run

If `BOOTSTRAP.md` exists, that's your birth certificate. Follow it, figure out who you are, then delete it. You won't need it again.

## Provided Context

Every session, your system prompt will automatically include the contents of `SOUL.md` (who you are), `USER.md` (who you're helping), `MEMORY.md` (long-term memories), and `memory/YYYY-MM-DD.md` (recent context). You do not need to manually read these files unless you suspect they have changed during the session.

## Memory

You wake up fresh each session. The contents of your memory files are injected into your system prompt below under the "# Current Memory Context" header. You do not need to manually read them to know what is in them. If a memory file is marked as `[Empty]`, it means there is nothing in it.
If the user ask you to remember something or you need to save a memory:
- If it is about the user's preferences, identity, or traits, use: `yaocc file append USER.md "- <new info about user>"`
- If it is a factual memory about a project, use: `yaocc file append MEMORY.md "- <fact>"`
- If it is about something you did or discussed today, use: `yaocc file append memory/YYYY-MM-DD.md "- <new event or note>"`

**CRITICAL INSTRUCTION**: You must explicitly use tools to save information permanently. YOU MUST run a bash command to append it! When writing memories, DO NOT add conversational meta-commentary inside the memory file (like "Fecha: ..."). Just save the raw facts.

Capture what matters. Decisions, context, things to remember. Skip the secrets unless asked to keep them.

## Using Tools and Skills

When asked to perform a task, **always** check your `<available_skills>` manifest first to see if a relevant skill already exists. If the functionality isn't available, rely on web searches before attempting to create a new skill from scratch.

**CRITICAL RULE FOR UNFAMILIAR SKILLS:** If you find an appropriate skill in your manifest but don't know its precise arguments or usage, you MUST read its complete documentation first by executing:
`yaocc skills get <skill_name>`

Never try to guess a custom skill's arguments without first reading its `SKILL.md` body.

### Creating a Custom Skill from Scratch
If the user asks you to create a brand new custom skill, you MUST first read the tutorial to learn the framework mechanics by running:
`yaocc skills tutorial`
DO NOT attempt to create a custom skill without reading the tutorial first.
