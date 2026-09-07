/**
 * Salesforce → callhook recipe.
 *
 * Two supported paths; both POST a callhook event to your server:
 *
 * 1. Flow / Apex trigger (this file) — synchronous callout on record change.
 * 2. Outbound Message → generic adapter — if you can't deploy Apex, point an
 *    Outbound Message at POST /integrations/generic/webhook with the
 *    X-Callhook-Secret header (needs a middleware that sets headers, since
 *    Outbound Messages can't add custom headers — see the webhook-settings
 *    workaround in the README).
 *
 * Deploy (Apex):
 *   sfdx force:apex:class:deploy -f CallhookCallout.cls -u <org>
 * Then call from a Flow "Apex Action" or a record-triggered Apex trigger.
 */
public with sharing class CallhookCallout {
    // Named Credential 'callhook' → your server URL (recommended), or hardcode.
    private static final String ENDPOINT = 'callhook/api/events';

    // Remote Site Setting required if not using a Named Credential.

    public class CallRequest {
        public String id;
        public String type;
        public String customer_id;
        public String phone;
        public String callback_url;
        public Map<String, Object> payload;
    }

    /** Fire one callhook event. Called from Flow or a trigger. */
    @future(callout=true)
    public static void fireEvent(
        String eventType, String customerId, String phone, String callbackUrl,
        Map<String, Object> payload
    ) {
        CallRequest reqBody = new CallRequest();
        reqBody.id = 'sf_' + String.valueOf(DateTime.now().getTime()) + '_' + customerId;
        reqBody.type = eventType;
        reqBody.customer_id = customerId;
        reqBody.phone = phone;
        reqBody.callback_url = callbackUrl;
        reqBody.payload = payload == null ? new Map<String, Object>() : payload;

        HttpRequest req = new HttpRequest();
        req.setEndpoint('call:' + ENDPOINT);
        req.setMethod('POST');
        req.setHeader('Content-Type', 'application/json');
        // Named Credential 'callhook' with the CALLHOOK_INTAKE_TOKEN as the
        // password provides the Authorization header automatically.
        req.setBody(JSON.serialize(reqBody));

        Http http = new Http();
        HttpResponse res = http.send(req);
        if (res.getStatusCode() >= 300) {
            System.debug(LoggingLevel.ERROR,
                'callhook callout failed: ' + res.getStatusCode() + ' ' + res.getBody());
        }
    }

    /** Example: opportunity stage → follow-up call. */
    public static void onStageChange(Opportunity opp) {
        String eventType = 'appointment.reminder';
        Map<String, Object> payload = new Map<String, Object>{
            'what' => 'your proposal for ' + opp.Name,
            'when' => 'this week'
        };
        fireEvent(eventType, opp.AccountId, null, null, payload);
    }
}

/*
 * Example trigger wiring:
 *
 * trigger OpportunityCallhook on Opportunity (after update) {
 *     for (Opportunity opp : Trigger.new) {
 *         Opportunity old = Trigger.oldMap.get(opp.Id);
 *         if (old.StageName != opp.StageName && opp.StageName == 'Negotiation') {
 *             CallhookCallout.onStageChange(opp);
 *         }
 *     }
 * }
 */
