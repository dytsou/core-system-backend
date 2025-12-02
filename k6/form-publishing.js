import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const publishSuccessRate = new Rate('publish_success');
const raceConditionRate = new Rate('publish_race_conditions');
const duplicatePublishRate = new Rate('duplicate_publishes');
const loginSuccessRate = new Rate('login_success');

// Test configuration
export const options = {
  setupTimeout: '120s', // Allow 2 minutes for setup (creating forms takes time)
  stages: [
    { duration: '30s', target: 5 },
    { duration: '1m', target: 10 },
    { duration: '1m', target: 0 },
  ],
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const USER_UUIDS = __ENV.USER_UUIDS ? __ENV.USER_UUIDS.split(',') : [];

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
  try {
    const res = http.post(
      `${BASE_URL}/api/auth/login/internal`,
      JSON.stringify({ uid: userUuid }),
      { headers: { 'Content-Type': 'application/json' }, timeout: '10s' }
    );

    if (res.status === 0) {
      // Connection failed (DNS, network, timeout, etc.)
      const errorMsg = res.error || 'Unknown connection error';
      if (errorMsg.includes('no such host') || errorMsg.includes('lookup')) {
        console.error(`DNS/Network error: Cannot resolve or reach ${BASE_URL}`);
      } else if (errorMsg.includes('timeout') || errorMsg.includes('deadline exceeded')) {
        console.error(`Request timeout: Server at ${BASE_URL} did not respond within 10s`);
      } else if (errorMsg.includes('connection reset') || errorMsg.includes('reset by peer')) {
        console.error(`Connection reset: Server at ${BASE_URL} closed the connection`);
      } else {
        console.error(`Connection failed: ${errorMsg}`);
      }
      return null;
    }

    if (res.status === 200) {
      const cookies = extractCookies(res.cookies);
      if (cookies['access_token']) {
        return { cookies };
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
    {
      headers: { 'Content-Type': 'application/json' },
      cookies,
      timeout: '30s',
    }
  );

  if (res.status === 201) {
    try {
      return JSON.parse(res.body);
    } catch (e) {
      console.error(`Failed to parse org response: ${res.body}`);
      return null;
    }
  }
  console.error(`Create org failed: status=${res.status}, body=${res.body}`);
  return null;
}

// Helper: create unit
function createUnit(cookies, orgSlug, name) {
  try {
    const res = http.post(
      `${BASE_URL}/api/orgs/${orgSlug}/units`,
      JSON.stringify({ name, description: 'Test unit for k6', metadata: {} }),
      {
        headers: { 'Content-Type': 'application/json' },
        cookies,
        timeout: '30s',
      }
    );

    if (res.status === 0) {
      const errorMsg = res.error || 'Unknown connection error';
      if (errorMsg.includes('timeout') || errorMsg.includes('deadline exceeded')) {
        console.error(`Create unit timeout: Request to ${BASE_URL}/api/orgs/${orgSlug}/units timed out after 30s`);
        console.error(`This may indicate the server is overloaded or the operation is taking too long`);
      } else if (errorMsg.includes('connection reset') || errorMsg.includes('reset by peer')) {
        console.error(`Create unit connection reset: Server closed the connection`);
      } else {
        console.error(`Create unit connection failed: ${errorMsg}`);
      }
      return null;
    }

    if (res.status === 201) {
      try {
        return JSON.parse(res.body);
      } catch (e) {
        console.error(`Failed to parse unit response: ${res.body}`);
        return null;
      }
    }
    console.error(`Create unit failed: status=${res.status}, body=${res.body}`);
    return null;
  } catch (e) {
    console.error(`Create unit exception: ${e.message}`);
    return null;
  }
}

// Helper: create form (draft)
function createForm(cookies, orgSlug, unitId, title) {
  const res = http.post(
    `${BASE_URL}/api/orgs/${orgSlug}/units/${unitId}/forms`,
    JSON.stringify({ title, description: 'Test form for k6 publishing', previewMessage: 'Preview', deadline: null }),
    {
      headers: { 'Content-Type': 'application/json' },
      cookies,
      timeout: '30s', // 30 second timeout per request
    }
  );

  if (res.status === 201) {
    try {
      return JSON.parse(res.body);
    } catch (e) {
      console.error(`Failed to parse form response: ${res.body}`);
      return null;
    }
  }
  console.error(`Create form failed: status=${res.status}, body=${res.body}`);
  return null;
}

// Helper: create question
function createQuestion(cookies, formId, title, order) {
  const res = http.post(
    `${BASE_URL}/api/forms/${formId}/questions`,
    JSON.stringify({ required: true, type: 'short_text', title, description: 'Test', order }),
    {
      headers: { 'Content-Type': 'application/json' },
      cookies,
      timeout: '30s', // 30 second timeout per request
    }
  );

  if (res.status === 201) {
    try {
      return JSON.parse(res.body);
    } catch (e) {
      console.error(`Failed to parse question response: ${res.body}`);
      return null;
    }
  }
  console.error(`Create question failed: status=${res.status}, body=${res.body}`);
  return null;
}

// Helper: get form
function getForm(cookies, formId) {
  const res = http.get(
    `${BASE_URL}/api/forms/${formId}`,
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  if (res.status === 200) {
    try {
      return JSON.parse(res.body);
    } catch (e) {
      return null;
    }
  }
  return null;
}

// Helper: publish form
function publishForm(cookies, formId, unitIds) {
  const res = http.post(
    `${BASE_URL}/api/forms/${formId}/publish`,
    JSON.stringify({ unitIds }),
    { headers: { 'Content-Type': 'application/json' }, cookies }
  );

  return { status: res.status, body: res.body };
}

// Setup - create org, unit, and multiple draft forms
export function setup() {
  if (USER_UUIDS.length === 0) {
    throw new Error('USER_UUIDS is required');
  }

  console.log(`Setting up test data for form publishing... (BASE_URL: ${BASE_URL})`);

  const auth = login(USER_UUIDS[0]);
  if (!auth) {
    const errorMsg = `Failed to login during setup. Cannot reach ${BASE_URL}. ` +
      `Please verify the server is running and accessible. ` +
      `If testing locally, try: BASE_URL=http://localhost:8080`;
    throw new Error(errorMsg);
  }

  const timestamp = Date.now();
  const orgSlug = `k6-pub-test-${timestamp}`;

  // Create org
  const org = createOrg(auth.cookies, `K6 Publish Test ${timestamp}`, orgSlug);
  if (!org) {
    throw new Error('Failed to create organization');
  }

  // Create units
  const unit1 = createUnit(auth.cookies, orgSlug, `Unit 1 ${timestamp}`);
  const unit2 = createUnit(auth.cookies, orgSlug, `Unit 2 ${timestamp}`);
  if (!unit1 || !unit2) {
    throw new Error('Failed to create units');
  }

  // Create multiple draft forms for testing
  const forms = [];
  const numForms = 5; // Reduced to 5 forms to speed up setup and reduce timeout risk

  for (let i = 0; i < numForms; i++) {
    console.log(`Creating form ${i + 1}/${numForms}...`);
    const form = createForm(auth.cookies, orgSlug, unit1.id, `Draft Form ${i} - ${timestamp}`);
    if (form) {
      // Add a question to each form
      const question = createQuestion(auth.cookies, form.id, `Question for form ${i}`, 1);
      if (question) {
        forms.push(form.id);
        console.log(`Created form ${i + 1} with question`);
      } else {
        console.warn(`Form ${i + 1} created but question failed - form will still work`);
        forms.push(form.id); // Add form anyway, question is optional
      }
    } else {
      console.error(`Failed to create form ${i + 1}`);
    }
  }

  if (forms.length === 0) {
    throw new Error('Failed to create any forms - cannot run test');
  }

  console.log(`Successfully created ${forms.length} forms (out of ${numForms} attempts)`);

  console.log(`Setup complete! Created ${forms.length} draft forms`);

  return {
    orgSlug,
    unitIds: [unit1.id, unit2.id],
    formIds: forms,
    formIndex: 0,
    userId: USER_UUIDS[0], // Store user UUID for cleanup
  };
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
    // Delete forms (this will cascade delete questions)
    if (data.formIds && Array.isArray(data.formIds)) {
      console.log(`Deleting ${data.formIds.length} forms...`);
      for (const formId of data.formIds) {
        deleteForm(auth.cookies, formId);
      }
    }

    // Delete units
    if (data.unitIds && Array.isArray(data.unitIds)) {
      console.log(`Deleting ${data.unitIds.length} units...`);
      for (const unitId of data.unitIds) {
        deleteUnit(auth.cookies, data.orgSlug, unitId);
      }
    }

    // Delete organization (final cleanup - should cascade delete everything)
    if (data.orgSlug) {
      console.log(`Deleting org: ${data.orgSlug}`);
      deleteOrg(auth.cookies, data.orgSlug);
    }

    console.log('Cleanup complete!');
  } catch (error) {
    console.error(`Error during cleanup: ${error.message}`);
  }
}

// Main test - each VU gets a different form
export default function (data) {
  const userUuid = USER_UUIDS[Math.floor(Math.random() * USER_UUIDS.length)];

  const auth = login(userUuid);
  if (!auth) {
    loginSuccessRate.add(0);
    check(false, { 'login succeeded': () => false });
    return;
  }
  loginSuccessRate.add(1);

  // Pick a form (round-robin)
  const formIndex = __VU % data.formIds.length;
  const formId = data.formIds[formIndex];

  // Check current form status
  const form = getForm(auth.cookies, formId);
  if (!form) {
    console.error(`Form ${formId} not found`);
    return;
  }

  if (form.status !== 'draft') {
    // Form already published, skip
    return;
  }

  // Simulate concurrent publish attempts
  const numConcurrentPublishes = 3;
  const publishResults = [];

  for (let i = 0; i < numConcurrentPublishes; i++) {
    const result = publishForm(auth.cookies, formId, data.unitIds);
    publishResults.push(result);

    if (result.status === 200) {
      publishSuccessRate.add(1);
    } else if (result.status === 400 || result.status === 409) {
      publishSuccessRate.add(0);
      raceConditionRate.add(1);
    } else {
      publishSuccessRate.add(0);
      console.error(`Unexpected publish status: ${result.status}, body: ${result.body}`);
    }

    sleep(0.1);
  }

  sleep(0.5);
  const finalForm = getForm(auth.cookies, formId);

  const successfulPublishes = publishResults.filter(r => r.status === 200).length;

  check(successfulPublishes, {
    'at most one publish succeeded': (count) => count <= 1,
    'form is published': () => finalForm && finalForm.status === 'published',
  });

  if (successfulPublishes > 1) {
    duplicatePublishRate.add(1);
    console.error(`Race condition: ${successfulPublishes} publishes succeeded`);
  } else {
    duplicatePublishRate.add(0);
  }

  sleep(1);
}

export function handleSummary(data) {
  return {
    'stdout': JSON.stringify(data, null, 2),
    'form-publishing-results.json': JSON.stringify(data, null, 2),
  };
}
