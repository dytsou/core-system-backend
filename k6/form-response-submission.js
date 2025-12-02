import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const duplicateResponseRate = new Rate('duplicate_responses');
const raceConditionRate = new Rate('race_conditions');
const loginSuccessRate = new Rate('login_success');

// Test configuration
export const options = {
  setupTimeout: '60s', // Allow 1 minute for setup
  stages: [
    { duration: '30s', target: 10 },   // Ramp up to 10 users
    { duration: '1m', target: 50 },    // Ramp up to 50 users
    { duration: '2m', target: 100 },   // Ramp up to 100 users
    { duration: '1m', target: 0 },     // Ramp down
  ],
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const USER_UUIDS = __ENV.USER_UUIDS ? __ENV.USER_UUIDS.split(',') : [];

// Helper: extract cookie values from k6 response cookies
function extractCookies(responseCookies) {
  const cookies = {};
  for (const name in responseCookies) {
    if (responseCookies[name] && responseCookies[name].length > 0) {
      cookies[name] = responseCookies[name][0].value;
    }
  }
  return cookies;
}

// Helper: check connectivity to BASE_URL
function checkConnectivity() {
  try {
    const res = http.get(`${BASE_URL}/health`, { timeout: '5s' });
    return res.status !== 0; // status 0 means connection failed
  } catch (e) {
    return false;
  }
}

// Helper: login and get JWT token
function login(userUuid) {
  try {
    if (!checkConnectivity()) {
      console.error(`Cannot reach ${BASE_URL}`);
      console.error(`Please check:`);
      console.error(`  1. Is BASE_URL correct? Current: ${BASE_URL}`);
      console.error(`  2. Is the server running and accessible?`);
      console.error(`  3. Do you need to use localhost instead? (e.g., http://localhost:8080)`);
      return null;
    }
    const res = http.post(
      `${BASE_URL}/api/auth/login/internal`,
      JSON.stringify({ uid: userUuid }),
      { headers: { 'Content-Type': 'application/json' }, timeout: '10s' }
    );

    if (res.status === 0) {
      // Connection failed (DNS, network, etc.)
      const errorMsg = res.error || 'Unknown connection error';
      if (errorMsg.includes('no such host') || errorMsg.includes('lookup')) {
        console.error(`DNS/Network error: Cannot resolve or reach ${BASE_URL}`);
        console.error(`Error details: ${errorMsg}`);
        console.error(`Please check:`);
        console.error(`  1. Is BASE_URL correct? Current: ${BASE_URL}`);
        console.error(`  2. Is the server running and accessible?`);
        console.error(`  3. Do you need to use localhost instead? (e.g., http://localhost:8080)`);
      } else {
        console.error(`Connection failed: ${errorMsg}`);
      }
      return null;
    }

    if (res.status === 200) {
      const cookies = extractCookies(res.cookies);
      if (cookies['access_token']) {
        return { cookies, accessToken: cookies['access_token'] };
      }
    }
    console.error(`Login failed: status=${res.status}, body=${res.body}`);
    return null;
  } catch (e) {
    console.error(`Login exception: ${e.message}`);
    return null;
  }
}

// Helper: create organization
function createOrg(cookies, name, slug) {
  const res = http.post(
    `${BASE_URL}/api/orgs`,
    JSON.stringify({ name, description: 'Test org for k6', slug, metadata: {} }),
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  if (res.status === 201) {
    return JSON.parse(res.body);
  } else if (res.status === 400 && res.body.includes('already exists')) {
    // Org already exists, get it
    const getRes = http.get(`${BASE_URL}/api/orgs/${slug}`, { cookies });
    if (getRes.status === 200) {
      return JSON.parse(getRes.body);
    }
  }
  console.error(`Create org failed: status=${res.status}, body=${res.body}`);
  return null;
}

// Helper: create unit under org
function createUnit(cookies, orgSlug, name) {
  const res = http.post(
    `${BASE_URL}/api/orgs/${orgSlug}/units`,
    JSON.stringify({ name, description: 'Test unit for k6', metadata: {} }),
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  if (res.status === 201) {
    return JSON.parse(res.body);
  }
  console.error(`Create unit failed: status=${res.status}, body=${res.body}`);
  return null;
}

// Helper: create form under unit
function createForm(cookies, orgSlug, unitId, title) {
  const res = http.post(
    `${BASE_URL}/api/orgs/${orgSlug}/units/${unitId}/forms`,
    JSON.stringify({ title, description: 'Test form for k6', previewMessage: 'Preview', deadline: null }),
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  if (res.status === 201) {
    return JSON.parse(res.body);
  }
  console.error(`Create form failed: status=${res.status}, body=${res.body}`);
  return null;
}

// Helper: create question for form
function createQuestion(cookies, formId, title, order) {
  const res = http.post(
    `${BASE_URL}/api/forms/${formId}/questions`,
    JSON.stringify({
      required: true,
      type: 'short_text',
      title,
      description: 'Test question',
      order,
    }),
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  if (res.status === 201) {
    return JSON.parse(res.body);
  }
  console.error(`Create question failed: status=${res.status}, body=${res.body}`);
  return null;
}

// Helper: publish form
function publishForm(cookies, formId, unitIds) {
  const res = http.post(
    `${BASE_URL}/api/forms/${formId}/publish`,
    JSON.stringify({ unitIds }),
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  if (res.status === 200) {
    return true;
  }
  console.error(`Publish form failed: status=${res.status}, body=${res.body}`);
  return false;
}

// Helper: delete question
function deleteQuestion(cookies, formId, questionId) {
  const res = http.del(
    `${BASE_URL}/api/forms/${formId}/questions/${questionId}`,
    null,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );
  return res.status === 200 || res.status === 204;
}

// Helper: delete form
function deleteForm(cookies, formId) {
  const res = http.del(
    `${BASE_URL}/api/forms/${formId}`,
    null,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );
  return res.status === 200 || res.status === 204;
}

// Helper: delete unit
function deleteUnit(cookies, orgSlug, unitId) {
  const res = http.del(
    `${BASE_URL}/api/orgs/${orgSlug}/units/${unitId}`,
    null,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );
  return res.status === 200 || res.status === 204;
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

// Setup function - runs once before all VUs
export function setup() {
  if (USER_UUIDS.length === 0) {
    throw new Error('USER_UUIDS is required. Set it in .env or pass via --env');
  }

  console.log(`Setting up test data... (BASE_URL: ${BASE_URL})`);

  // Login with first user
  const auth = login(USER_UUIDS[0]);
  if (!auth) {
    const errorMsg = `Failed to login during setup. Cannot reach ${BASE_URL}. ` +
      `Please verify the server is running and accessible. ` +
      `If testing locally, try: BASE_URL=http://localhost:8080`;
    throw new Error(errorMsg);
  }

  const timestamp = Date.now();
  const orgSlug = `k6-test-org-${timestamp}`;
  const orgName = `K6 Test Org ${timestamp}`;

  // Create organization
  console.log(`Creating org: ${orgSlug}`);
  const org = createOrg(auth.cookies, orgName, orgSlug);
  if (!org) {
    throw new Error('Failed to create organization');
  }
  console.log(`Created org: ${org.id}`);

  // Create unit
  console.log('Creating unit...');
  const unit = createUnit(auth.cookies, orgSlug, `K6 Test Unit ${timestamp}`);
  if (!unit) {
    throw new Error('Failed to create unit');
  }
  console.log(`Created unit: ${unit.id}`);

  // Create form
  console.log('Creating form...');
  const form = createForm(auth.cookies, orgSlug, unit.id, `K6 Test Form ${timestamp}`);
  if (!form) {
    throw new Error('Failed to create form');
  }
  console.log(`Created form: ${form.id}`);

  // Create questions
  console.log('Creating questions...');
  const q1 = createQuestion(auth.cookies, form.id, 'Test Question 1', 1);
  const q2 = createQuestion(auth.cookies, form.id, 'Test Question 2', 2);

  if (!q1 || !q2) {
    throw new Error('Failed to create questions');
  }
  console.log(`Created questions: ${q1.id}, ${q2.id}`);

  // Publish form (required for submission)
  console.log('Publishing form...');
  const published = publishForm(auth.cookies, form.id, [unit.id]);
  if (!published) {
    console.warn('Form publish failed - submissions may fail');
  }

  console.log('Setup complete!');

  return {
    formId: form.id,
    questionId1: q1.id,
    questionId2: q2.id,
    orgSlug: orgSlug,
    unitId: unit.id,
    userId: USER_UUIDS[0], // Store user UUID for cleanup
  };
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

  // Delete in reverse order: questions -> form -> unit -> org
  try {
    // Delete questions
    if (data.questionId1) {
      console.log(`Deleting question: ${data.questionId1}`);
      deleteQuestion(auth.cookies, data.formId, data.questionId1);
    }
    if (data.questionId2) {
      console.log(`Deleting question: ${data.questionId2}`);
      deleteQuestion(auth.cookies, data.formId, data.questionId2);
    }

    // Delete form (this may cascade delete questions, but we tried to delete them first)
    if (data.formId) {
      console.log(`Deleting form: ${data.formId}`);
      deleteForm(auth.cookies, data.formId);
    }

    // Delete unit
    if (data.unitId && data.orgSlug) {
      console.log(`Deleting unit: ${data.unitId}`);
      deleteUnit(auth.cookies, data.orgSlug, data.unitId);
    }

    // Delete organization (this may cascade delete units and forms, but we tried to delete them first)
    if (data.orgSlug) {
      console.log(`Deleting org: ${data.orgSlug}`);
      deleteOrg(auth.cookies, data.orgSlug);
    }

    console.log('Cleanup complete!');
  } catch (error) {
    console.error(`Error during cleanup: ${error.message}`);
  }
}

// Helper: submit form response
function submitFormResponse(cookies, formId, questionId1, questionId2, userId) {
  const answers = [];
  if (questionId1) {
    answers.push({ questionId: questionId1, value: `Response from ${userId} at ${Date.now()}` });
  }
  if (questionId2) {
    answers.push({ questionId: questionId2, value: `Answer 2 from ${userId}` });
  }

  const res = http.post(
    `${BASE_URL}/api/forms/${formId}/responses`,
    JSON.stringify({ answers }),
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  return { status: res.status, body: res.body };
}

// Helper: get form responses
function getFormResponses(cookies, formId) {
  const res = http.get(
    `${BASE_URL}/api/forms/${formId}/responses`,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  if (res.status === 200) {
    try {
      const data = JSON.parse(res.body);
      return Array.isArray(data) ? data : [];
    } catch (e) {
      return [];
    }
  }
  return [];
}

// Main test function
export default function (data) {
  const userUuid = USER_UUIDS[Math.floor(Math.random() * USER_UUIDS.length)];

  const auth = login(userUuid);
  if (!auth) {
    loginSuccessRate.add(0);
    check(false, { 'login succeeded': () => false });
    return;
  }
  loginSuccessRate.add(1);

  check(auth.accessToken, {
    'access token received': (token) => token && token.length > 0,
  });

  // Submit form responses (simulating race condition)
  const numConcurrentSubmissions = 3;

  for (let i = 0; i < numConcurrentSubmissions; i++) {
    const result = submitFormResponse(
      auth.cookies,
      data.formId,
      data.questionId1,
      data.questionId2,
      `${userUuid}-${i}`
    );

    if (result.status === 201) {
      sleep(0.1);
      const responses = getFormResponses(auth.cookies, data.formId);
      if (Array.isArray(responses)) {
        const userResponses = responses.filter(r => r.userId === userUuid);
        if (userResponses.length > 1) {
          duplicateResponseRate.add(1);
          raceConditionRate.add(1);
          console.error(`Race condition: ${userResponses.length} responses for user ${userUuid}`);
        } else {
          duplicateResponseRate.add(0);
          raceConditionRate.add(0);
        }
      }
    } else if (result.status === 400 || result.status === 409) {
      duplicateResponseRate.add(0);
    } else {
      console.error(`Unexpected status ${result.status}: ${result.body}`);
    }
  }

  sleep(0.5);
  const finalResponses = getFormResponses(auth.cookies, data.formId);
  const userFinalResponses = Array.isArray(finalResponses)
    ? finalResponses.filter(r => r.userId === userUuid)
    : [];

  check(userFinalResponses.length, {
    'at most one response per user': (count) => count <= 1,
  });

  sleep(1);
}

export function handleSummary(data) {
  return {
    'stdout': JSON.stringify(data, null, 2),
    'form-response-submission-results.json': JSON.stringify(data, null, 2),
  };
}
