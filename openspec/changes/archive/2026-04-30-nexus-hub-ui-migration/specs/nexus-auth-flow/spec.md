# Spec: Nexus Auth Flow

## Overview
Redesign the authentication experience from dark minimal OTP flow to a polished, light-mode Nexus Hub enterprise auth experience with workspace selection and onboarding.

## Screens

### Login (`/_auth/login`)
- Centered white card on radial gradient blue-white background
- Blue Nexus icon at top
- "Welcome back to Nexus" h2 heading
- "Log in to continue your secure workspace." body-md muted text
- Single input: "Email or Phone Number" with placeholder
- "Continue →" primary button (full width)
- "or continue with" divider
- Google sign-in button (outline, Google icon)
- SSO button (outline, key icon)
- Footer: "Privacy Policy · Terms of Service" links

### Verification (`/_auth/verify`)
- Same centered card layout
- Blue shield icon
- "Verify your identity" heading
- "Enter the 6-digit code sent to {email/phone}" body text
- 6 individual digit inputs (48px × 56px each, gap-3)
- Auto-advance on digit entry, auto-submit on 6th digit
- "Didn't receive a code? Resend" link with countdown timer
- "← Back to login" link

### Welcome Back (`/_auth/welcome`)
- Full-screen centered layout
- Nexus icon (animated pulse)
- "Welcome back, {displayName}!" h2
- Animated progress bar (indeterminate)
- Auto-redirect to workspace-select after 2s or when workspaces loaded

### Workspace Selection (`/_auth/workspace-select`)
- Centered card (max-w-2xl)
- Blue briefcase icon
- "Select your workspace" h1
- "Choose where you want to go. You can switch between workspaces at any time." body text
- Two columns: "Organizations" | "Personal"
- Each workspace: avatar/icon + name + tier badge + member count + chevron-right
- Clicking a workspace → navigate to `/_workspace?ws={id}`
- Bottom row: "Join an Organization" outline button + "+ Create Workspace" primary button

### Onboarding Step 1 (`/_auth/onboarding`)
- Progress: "STEP 1 OF 2" label-caps + progress bar + "NEXT: INVITE TEAM" label
- "Create your Organization" h1
- "Set up your workspace. You can always change these details later." body text
- Logo upload area: circle avatar placeholder + "Upload Image" outline button
- Company Name input (required)
- Workspace URL: text input + ".enterpriseflow.com" suffix
- "Next Step →" primary button

### Onboarding Step 2 (`/_auth/onboarding`)
- Progress: "STEP 2 OF 2" + "FINISH" label
- "Invite your team" h1
- "Add team members to get started" body text
- Email input rows: email input + role dropdown + X remove button
- "+ Add another" link
- "Skip for now" outline button + "Create Organization →" primary button
- On submit: calls CreateWorkspace + InviteMember APIs

## Behavior
- Login checks if email/phone exists → if yes, send OTP → verify screen
- If no account, redirect to register screen (password + display name)
- After successful auth, ALWAYS go to workspace-select (not auto-pick)
- Onboarding creates `type=organization` workspace, then redirects to workspace-select
- Personal workspace is auto-created on registration with `type=personal`
