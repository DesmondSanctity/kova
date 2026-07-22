/**
 * Kredo SDK — drop-in credit scoring widget + client for any JS/TS frontend.
 *
 *   import { Kredo } from "@kredo/sdk";
 *   Kredo.init({ key: "pk_live_...", apiBase: "https://api.kredo.dev" }).open();
 *
 * Link mode (individual lenders): mount against a request the borrower opens.
 *   Kredo.init({ requestId: "abc", container: el, autoOpen: true });
 */
export interface Reason {
    factor: string;
    impact: 'positive' | 'negative';
    detail: string;
}
export interface ScoreFeatures {
    monthsCovered: number;
    transactionCount: number;
    totalInflow: number;
    netCashflow: number;
    avgMonthlyInflow: number;
    inflowRegularity: number;
    distinctIncomeSources: number;
    debtRatio: number;
}
export interface ScoreResult {
    score: number;
    band: string;
    confidence: number;
    limitRecommendation: number;
    reasons: Reason[];
    features: ScoreFeatures;
}
export interface ScoreReport {
    files: {
        filename: string;
        bank?: string;
        parsed: boolean;
        transactions: number;
        error?: string;
    }[];
    accounts: string[];
    banks: string[];
    score: ScoreResult;
}
export interface Bank {
    name: string;
    slug: string;
    code: string;
    logo?: string;
    supported?: boolean;
}
export interface KredoOptions {
    /** Publishable key (pk_...). Required unless `requestId` is set. */
    key?: string;
    /** Assessment request id (link mode). No key needed — the link is the capability. */
    requestId?: string;
    /** Base URL of the Kredo API. Defaults to the current origin. */
    apiBase?: string;
    /** Mount inline into this element instead of a modal overlay. */
    container?: HTMLElement;
    theme?: 'dark' | 'light';
    autoOpen?: boolean;
    onResult?: (report: ScoreReport) => void;
    /** Fired in link mode when statements are received (scoring happens async; the result is emailed). */
    onSubmitted?: () => void;
    onError?: (error: Error) => void;
}
export interface KredoInstance {
    open(): void;
    close(): void;
    destroy(): void;
}
export interface KredoClientOptions {
    /** Base URL of the Kredo API. Defaults to the current origin. */
    apiBase?: string;
    /** Secret key (sk_...) — required for creating links, listing and rejecting. */
    key: string;
}
/**
 * Programmatic client for creditworthiness assessment: create borrower links,
 * read scored results, and reject offers. Disbursement is a first-party
 * dashboard action (it moves money from your connected Monnify wallet) and is
 * intentionally not part of the API/SDK.
 */
export declare class KredoClient {
    private base;
    private key;
    constructor(options: KredoClientOptions);
    private req;
    /** Create a shareable borrower link. */
    createLink(note?: string): Promise<{
        id: string;
        borrowerUrl: string;
        viewUrl: string;
    }>;
    /** List all links/requests for the workspace. */
    listLinks(): Promise<{
        links: any[];
    }>;
    /** Read a single request (status, decision, offer, score report). */
    getRequest(id: string): Promise<any>;
    /** Reject a scored/accepted request before payout. */
    reject(id: string): Promise<{
        status: string;
    }>;
}
export declare const Kredo: {
    init(options: KredoOptions): KredoInstance;
    /** Server/programmatic client for creditworthiness assessment (use a secret key). */
    client(options: KredoClientOptions): KredoClient;
};
export default Kredo;
