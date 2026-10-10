#!/bin/bash
cd /tmp/claude-1000/-home-zane-Desktop-projects-nexus-enterprise/01af34f8-1063-4799-8a72-b7e3df68cbcd/scratchpad/e2e/t
echo "== before"
SP_DIR=/tmp/claude-1000/-home-zane-Desktop-projects-nexus-enterprise/01af34f8-1063-4799-8a72-b7e3df68cbcd/scratchpad/e2e/t/.. timeout 300 node approval-api.mjs > /tmp/claude-1000/-home-zane-Desktop-projects-nexus-enterprise/01af34f8-1063-4799-8a72-b7e3df68cbcd/scratchpad/e2e/t/appr-before.log 2>&1
cp /home/zane/Desktop/projects/nexus-enterprise/plans/261010-2205-first-release/reports/e2e/approval-api.json /tmp/claude-1000/-home-zane-Desktop-projects-nexus-enterprise/01af34f8-1063-4799-8a72-b7e3df68cbcd/scratchpad/e2e/t/appr-before.json
docker restart nexus-e2e-policy-1 nexus-e2e-policy-read-1 >/dev/null
for i in $(seq 1 60); do a=$(docker inspect nexus-e2e-policy-1 --format '{{.State.Health.Status}}'); b=$(docker inspect nexus-e2e-policy-read-1 --format '{{.State.Health.Status}}'); [ "$a" = healthy ] && [ "$b" = healthy ] && break; sleep 2; done
echo "policy health: $a / $b"
echo "== after"
SP_DIR=/tmp/claude-1000/-home-zane-Desktop-projects-nexus-enterprise/01af34f8-1063-4799-8a72-b7e3df68cbcd/scratchpad/e2e/t/.. timeout 300 node approval-api.mjs > /tmp/claude-1000/-home-zane-Desktop-projects-nexus-enterprise/01af34f8-1063-4799-8a72-b7e3df68cbcd/scratchpad/e2e/t/appr-after.log 2>&1
cp /home/zane/Desktop/projects/nexus-enterprise/plans/261010-2205-first-release/reports/e2e/approval-api.json /tmp/claude-1000/-home-zane-Desktop-projects-nexus-enterprise/01af34f8-1063-4799-8a72-b7e3df68cbcd/scratchpad/e2e/t/appr-after.json
echo done
