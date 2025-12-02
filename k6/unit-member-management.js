import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const memberAddSuccessRate = new Rate('member_add_success');
const memberRemoveSuccessRate = new Rate('member_remove_success');
const concurrentOperationRate = new Rate('concurrent_operation_issues');
const loginSuccessRate = new Rate('login_success');

// Test configuration
export const options = {
  setupTimeout: '60s', // Allow 1 minute for setup
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 20 },
    { duration: '1m', target: 0 },
  ]
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const ADMIN_UUIDS = __ENV.ADMIN_UUIDS ? __ENV.ADMIN_UUIDS.split(',') : [];
const MEMBER_EMAILS = __ENV.MEMBER_EMAILS ? __ENV.MEMBER_EMAILS.split(',') : ['test-member@example.com'];

// Helper: extract cookie values
function extractCookies(responseCookies) {
  const cookies = {};
  for (const name in responseCookies) {
    if (responseCookies[name] && responseCookies[name].length > 0) {
      cookies[name] = responseCookies[name][0].value;
    }
  }
  return cookies;
}

// Helper: login
function login(userUuid) {
  const res = http.post(
    `${BASE_URL}/api/auth/login/internal`,
    JSON.stringify({ uid: userUuid }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  if (res.status === 200) {
    const cookies = extractCookies(res.cookies);
    if (cookies['access_token']) {
      return { cookies };
    }
  }
  console.error(`Login failed: status=${res.status}, body=${res.body}`);
  return null;
}

// Helper: create organization
function createOrg(cookies, name, slug) {
  const res = http.post(
    `${BASE_URL}/api/orgs`,
    JSON.stringify({ name, description: 'Test org for k6 member mgmt', slug, metadata: {} }),
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  if (res.status === 201) {
    return JSON.parse(res.body);
  }
  console.error(`Create org failed: status=${res.status}, body=${res.body}`);
  return null;
}

// Helper: add member
function addOrgMember(cookies, orgSlug, memberEmail) {
  const res = http.post(
    `${BASE_URL}/api/orgs/${orgSlug}/members`,
    JSON.stringify({ email: memberEmail.trim() }),
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  let memberId = null;
  if (res.status === 201) {
    try {
      const body = JSON.parse(res.body);
      memberId = body.member?.id;
    } catch (e) { }
  }

  return { status: res.status, body: res.body, memberId };
}

// Helper: remove member
function removeOrgMember(cookies, orgSlug, memberId) {
  const res = http.del(
    `${BASE_URL}/api/orgs/${orgSlug}/members/${memberId}`,
    null,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  return { status: res.status, body: res.body };
}

// Helper: list members
function listOrgMembers(cookies, orgSlug) {
  const res = http.get(
    `${BASE_URL}/api/orgs/${orgSlug}/members`,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  if (res.status === 200) {
    try {
      return JSON.parse(res.body) || [];
    } catch (e) {
      return [];
    }
  }
  return [];
}

// Setup - create org
export function setup() {
  if (ADMIN_UUIDS.length === 0) {
    throw new Error('ADMIN_UUIDS is required');
  }

  console.log('Setting up test data for member management...');

  const auth = login(ADMIN_UUIDS[0]);
  if (!auth) {
    throw new Error('Failed to login during setup');
  }

  const timestamp = Date.now();
  const orgSlug = `k6-member-test-${timestamp}`;

  const org = createOrg(auth.cookies, `K6 Member Test ${timestamp}`, orgSlug);
  if (!org) {
    throw new Error('Failed to create organization');
  }

  console.log(`Setup complete! Created org: ${orgSlug}`);

  return {
    orgSlug,
    userId: ADMIN_UUIDS[0], // Store user UUID for cleanup
  };
}

// Helper: delete organization
function deleteOrg(cookies, orgSlug) {
  const res = http.del(
    `${BASE_URL}/api/orgs/${orgSlug}`,
    null,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );
  return res.status === 200 || res.status === 204;
}

// Teardown function - runs once after all VUs finish
export function teardown(data) {
  if (!data) {
    console.log('No test data to clean up');
    return;
  }

  console.log('Cleaning up test data...');

  // Login with the same user used in setup
  const auth = login(data.userId);
  if (!auth) {
    console.error('Failed to login during teardown - skipping cleanup');
    return;
  }

  try {
    // Delete organization
    if (data.orgSlug) {
      console.log(`Deleting org: ${data.orgSlug}`);
      deleteOrg(auth.cookies, data.orgSlug);
      console.log('Cleanup complete!');
    }
  } catch (error) {
    console.error(`Error during cleanup: ${error.message}`);
  }
}

// Main test
export default function (data) {
  const adminUuid = ADMIN_UUIDS[Math.floor(Math.random() * ADMIN_UUIDS.length)];
  const memberEmail = MEMBER_EMAILS[Math.floor(Math.random() * MEMBER_EMAILS.length)];

  const auth = login(adminUuid);
  if (!auth) {
    loginSuccessRate.add(0);
    check(false, { 'login succeeded': () => false });
    return;
  }
  loginSuccessRate.add(1);

  // Test concurrent member additions
  const numConcurrentAdds = 3;
  const addResults = [];

  for (let i = 0; i < numConcurrentAdds; i++) {
    const result = addOrgMember(auth.cookies, data.orgSlug, memberEmail);
    addResults.push(result);

    if (result.status === 201) {
      memberAddSuccessRate.add(1);
    } else if (result.status === 409 || result.status === 400) {
      memberAddSuccessRate.add(0);
      concurrentOperationRate.add(1);
    } else {
      memberAddSuccessRate.add(0);
      console.error(`Unexpected add status: ${result.status}, body: ${result.body}`);
    }

    sleep(0.1);
  }

  sleep(0.5);
  const members = listOrgMembers(auth.cookies, data.orgSlug);

  // Find matching members
  const matchingMembers = Array.isArray(members)
    ? members.filter(m => m.member?.emails?.includes(memberEmail.trim()))
    : [];

  check(matchingMembers.length, {
    'member added only once': (count) => count <= 1,
  });

  // Test concurrent removals
  if (matchingMembers.length > 0) {
    const memberId = matchingMembers[0].member?.id;

    if (memberId) {
      const numConcurrentRemoves = 2;
      const removeResults = [];

      for (let i = 0; i < numConcurrentRemoves; i++) {
        const result = removeOrgMember(auth.cookies, data.orgSlug, memberId);
        removeResults.push(result);

        if (result.status === 200 || result.status === 204) {
          memberRemoveSuccessRate.add(1);
        } else if (result.status === 404) {
          memberRemoveSuccessRate.add(0);
          concurrentOperationRate.add(1);
        } else {
          memberRemoveSuccessRate.add(0);
        }

        sleep(0.1);
      }

      sleep(0.5);
      const membersAfter = listOrgMembers(auth.cookies, data.orgSlug);
      const matchingAfter = Array.isArray(membersAfter)
        ? membersAfter.filter(m => m.member?.emails?.includes(memberEmail.trim()))
        : [];

      check(matchingAfter.length, {
        'member removed successfully': (count) => count === 0,
      });
    }
  }

  sleep(1);
}

export function handleSummary(data) {
  return {
    'stdout': JSON.stringify(data, null, 2),
    'unit-member-management-results.json': JSON.stringify(data, null, 2),
  };
}
