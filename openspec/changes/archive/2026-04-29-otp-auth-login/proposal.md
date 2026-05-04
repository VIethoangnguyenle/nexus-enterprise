# OTP Auth Login (Lark-style)

## Problem

Current login uses username + password — not production-ready. No phone/email identity, no OTP verification, and the login UI is a basic form that doesn't match the Lark-inspired platform design.

## Solution

Replace the login flow with a 2-step OTP-based authentication inspired by Lark:
1. **Step 1**: Enter email or phone number (tabbed UI)
2. **Step 2**: Enter 4-digit OTP code

OTP is hardcoded to `9999` for development — real SMS/email integration deferred to later.

## Scope

### In scope
- New API endpoints: `POST /auth/otp/request` + `POST /auth/otp/verify`
- OTP session storage in Redis (5min TTL)
- Auto-register: new users created automatically on first OTP verify
- Auto-provision workspace + #general channel for new users
- DB migration: add `phone` column to `users` table
- New Lark-style split login UI (illustration left, form right)
- Tabbed input: "Email Address" | "Phone Number"
- Phone validation: Vietnam +84 format, 9-11 digits
- Email validation: standard format check
- 4-digit OTP input with auto-advance and auto-submit

### Out of scope
- Real SMS/email sending (OTP hardcoded to 9999)
- SSO, Google, Apple login buttons (cosmetic placeholders ok)
- Password-based login removal (keep legacy routes for backward compat)
- Rate limiting (future)
- Multi-country phone support beyond +84

## Impact
- **Breaking**: Frontend login flow changes completely
- **Non-breaking**: Legacy `/auth/login` + `/auth/register` routes preserved
- **DB**: Additive migration only (`ALTER TABLE ADD COLUMN`)
