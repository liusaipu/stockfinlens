export namespace ai_researcher {
	
	export class AIConfig {
	    enabled: boolean;
	    llm_provider: string;
	    llm_api_key: string;
	    llm_base_url: string;
	    llm_model: string;
	    llm_timeout: number;
	    temperature: number;
	    max_tokens: number;
	    top_p: number;
	    search_provider: string;
	    search_api_key: string;
	    search_depth: string;
	    search_timeout: number;
	    max_results: number;
	    search_recency_days: number;
	    focus_regions: string[];
	    output_language: string;
	    enable_social: boolean;
	    cache_ttl_hours: number;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.llm_provider = source["llm_provider"];
	        this.llm_api_key = source["llm_api_key"];
	        this.llm_base_url = source["llm_base_url"];
	        this.llm_model = source["llm_model"];
	        this.llm_timeout = source["llm_timeout"];
	        this.temperature = source["temperature"];
	        this.max_tokens = source["max_tokens"];
	        this.top_p = source["top_p"];
	        this.search_provider = source["search_provider"];
	        this.search_api_key = source["search_api_key"];
	        this.search_depth = source["search_depth"];
	        this.search_timeout = source["search_timeout"];
	        this.max_results = source["max_results"];
	        this.search_recency_days = source["search_recency_days"];
	        this.focus_regions = source["focus_regions"];
	        this.output_language = source["output_language"];
	        this.enable_social = source["enable_social"];
	        this.cache_ttl_hours = source["cache_ttl_hours"];
	    }
	}
	export class ResearchSource {
	    title: string;
	    url: string;
	    date: string;
	
	    static createFrom(source: any = {}) {
	        return new ResearchSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.url = source["url"];
	        this.date = source["date"];
	    }
	}
	export class ResearchSection {
	    title: string;
	    summary: string;
	    key_points: string[];
	    sentiment: string;
	
	    static createFrom(source: any = {}) {
	        return new ResearchSection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.key_points = source["key_points"];
	        this.sentiment = source["sentiment"];
	    }
	}
	export class AIResearchReport {
	    symbol: string;
	    name: string;
	    generated_at: string;
	    model_used: string;
	    from_cache: boolean;
	    sections: ResearchSection[];
	    sources: ResearchSource[];
	
	    static createFrom(source: any = {}) {
	        return new AIResearchReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.generated_at = source["generated_at"];
	        this.model_used = source["model_used"];
	        this.from_cache = source["from_cache"];
	        this.sections = this.convertValues(source["sections"], ResearchSection);
	        this.sources = this.convertValues(source["sources"], ResearchSource);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class TestConnectionResult {
	    success: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new TestConnectionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	    }
	}

}

export namespace analyzer {
	
	export class MetricChange {
	    name: string;
	    previous: number;
	    current: number;
	    delta: number;
	    deltaPct: number;
	    significant: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MetricChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.previous = source["previous"];
	        this.current = source["current"];
	        this.delta = source["delta"];
	        this.deltaPct = source["deltaPct"];
	        this.significant = source["significant"];
	    }
	}
	export class RiskAlertFlag {
	    code: string;
	    name: string;
	    value: number;
	    format: string;
	    level: string;
	    source: string;
	    details: string[];
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new RiskAlertFlag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.value = source["value"];
	        this.format = source["format"];
	        this.level = source["level"];
	        this.source = source["source"];
	        this.details = source["details"];
	        this.note = source["note"];
	    }
	}
	export class AnalysisDiff {
	    hasPrevious: boolean;
	    previousTime?: string;
	    currentTime?: string;
	    scoreChange: number;
	    gradeChanged: boolean;
	    previousGrade?: string;
	    currentGrade?: string;
	    newFlags?: RiskAlertFlag[];
	    resolvedFlags?: RiskAlertFlag[];
	    persistentFlags?: RiskAlertFlag[];
	    keyMetricChanges?: MetricChange[];
	
	    static createFrom(source: any = {}) {
	        return new AnalysisDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasPrevious = source["hasPrevious"];
	        this.previousTime = source["previousTime"];
	        this.currentTime = source["currentTime"];
	        this.scoreChange = source["scoreChange"];
	        this.gradeChanged = source["gradeChanged"];
	        this.previousGrade = source["previousGrade"];
	        this.currentGrade = source["currentGrade"];
	        this.newFlags = this.convertValues(source["newFlags"], RiskAlertFlag);
	        this.resolvedFlags = this.convertValues(source["resolvedFlags"], RiskAlertFlag);
	        this.persistentFlags = this.convertValues(source["persistentFlags"], RiskAlertFlag);
	        this.keyMetricChanges = this.convertValues(source["keyMetricChanges"], MetricChange);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InteractQA {
	    question: string;
	    answer: string;
	    questioner: string;
	    date: string;
	    answerDate: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new InteractQA(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.question = source["question"];
	        this.answer = source["answer"];
	        this.questioner = source["questioner"];
	        this.date = source["date"];
	        this.answerDate = source["answerDate"];
	        this.source = source["source"];
	    }
	}
	export class TTMMetrics {
	    hasData: boolean;
	    revenue: number;
	    netProfit: number;
	    operatingCash: number;
	    roe: number;
	    netMargin: number;
	    cashRatio: number;
	    mode: string;
	    endPeriod: string;
	    notes?: string;
	    periodCount: number;
	    periods: string[];
	
	    static createFrom(source: any = {}) {
	        return new TTMMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasData = source["hasData"];
	        this.revenue = source["revenue"];
	        this.netProfit = source["netProfit"];
	        this.operatingCash = source["operatingCash"];
	        this.roe = source["roe"];
	        this.netMargin = source["netMargin"];
	        this.cashRatio = source["cashRatio"];
	        this.mode = source["mode"];
	        this.endPeriod = source["endPeriod"];
	        this.notes = source["notes"];
	        this.periodCount = source["periodCount"];
	        this.periods = source["periods"];
	    }
	}
	export class QuarterlyAlertItem {
	    period: string;
	    previousPeriod: string;
	    metric: string;
	    current: number;
	    previous: number;
	    changePct: number;
	    level: string;
	    description: string;
	    compareType: string;
	
	    static createFrom(source: any = {}) {
	        return new QuarterlyAlertItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.period = source["period"];
	        this.previousPeriod = source["previousPeriod"];
	        this.metric = source["metric"];
	        this.current = source["current"];
	        this.previous = source["previous"];
	        this.changePct = source["changePct"];
	        this.level = source["level"];
	        this.description = source["description"];
	        this.compareType = source["compareType"];
	    }
	}
	export class QuarterlyAlert {
	    hasData: boolean;
	    items: QuarterlyAlertItem[];
	
	    static createFrom(source: any = {}) {
	        return new QuarterlyAlert(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasData = source["hasData"];
	        this.items = this.convertValues(source["items"], QuarterlyAlertItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RiskAlertSummary {
	    level: string;
	    score: number;
	    oneVeto: boolean;
	    flags: RiskAlertFlag[];
	    primaryMsg: string;
	
	    static createFrom(source: any = {}) {
	        return new RiskAlertSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.score = source["score"];
	        this.oneVeto = source["oneVeto"];
	        this.flags = this.convertValues(source["flags"], RiskAlertFlag);
	        this.primaryMsg = source["primaryMsg"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RIMScenario {
	    ROE: number;
	    Value: number;
	    DiffPct: number;
	    Grade: string;
	
	    static createFrom(source: any = {}) {
	        return new RIMScenario(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ROE = source["ROE"];
	        this.Value = source["Value"];
	        this.DiffPct = source["DiffPct"];
	        this.Grade = source["Grade"];
	    }
	}
	export class RIMYearDetail {
	    Year: number;
	    CalendarYear: number;
	    EPS: number;
	    DPS: number;
	    BPS: number;
	    RE: number;
	    Discount: number;
	    PVRE: number;
	
	    static createFrom(source: any = {}) {
	        return new RIMYearDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Year = source["Year"];
	        this.CalendarYear = source["CalendarYear"];
	        this.EPS = source["EPS"];
	        this.DPS = source["DPS"];
	        this.BPS = source["BPS"];
	        this.RE = source["RE"];
	        this.Discount = source["Discount"];
	        this.PVRE = source["PVRE"];
	    }
	}
	export class RIMResult {
	    Details: RIMYearDetail[];
	    SumPVRE: number;
	    CV: number;
	    PVCV: number;
	    Value: number;
	    Upside: number;
	    Pessimistic: RIMScenario;
	    Baseline: RIMScenario;
	    Optimistic: RIMScenario;
	
	    static createFrom(source: any = {}) {
	        return new RIMResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Details = this.convertValues(source["Details"], RIMYearDetail);
	        this.SumPVRE = source["SumPVRE"];
	        this.CV = source["CV"];
	        this.PVCV = source["PVCV"];
	        this.Value = source["Value"];
	        this.Upside = source["Upside"];
	        this.Pessimistic = this.convertValues(source["Pessimistic"], RIMScenario);
	        this.Baseline = this.convertValues(source["Baseline"], RIMScenario);
	        this.Optimistic = this.convertValues(source["Optimistic"], RIMScenario);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RIMForecast {
	    EPS: number[];
	    DPS: number[];
	    Years: string[];
	
	    static createFrom(source: any = {}) {
	        return new RIMForecast(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.EPS = source["EPS"];
	        this.DPS = source["DPS"];
	        this.Years = source["Years"];
	    }
	}
	export class RIMParams {
	    BPS0: number;
	    KE: number;
	    GTerminal: number;
	    Forecast: RIMForecast;
	    CurrentPrice: number;
	
	    static createFrom(source: any = {}) {
	        return new RIMParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.BPS0 = source["BPS0"];
	        this.KE = source["KE"];
	        this.GTerminal = source["GTerminal"];
	        this.Forecast = this.convertValues(source["Forecast"], RIMForecast);
	        this.CurrentPrice = source["CurrentPrice"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RIMData {
	    hasData: boolean;
	    params: RIMParams;
	    result?: RIMResult;
	    error?: string;
	    epsRaw?: Record<string, number>;
	    rf: number;
	    beta: number;
	    rmRf: number;
	
	    static createFrom(source: any = {}) {
	        return new RIMData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasData = source["hasData"];
	        this.params = this.convertValues(source["params"], RIMParams);
	        this.result = this.convertValues(source["result"], RIMResult);
	        this.error = source["error"];
	        this.epsRaw = source["epsRaw"];
	        this.rf = source["rf"];
	        this.beta = source["beta"];
	        this.rmRf = source["rmRf"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Deduction {
	    StepNum: number;
	    StepName: string;
	    Reason: string;
	    Points: number;
	
	    static createFrom(source: any = {}) {
	        return new Deduction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.StepNum = source["StepNum"];
	        this.StepName = source["StepName"];
	        this.Reason = source["Reason"];
	        this.Points = source["Points"];
	    }
	}
	export class YearScore {
	    Year: string;
	    RawScore: number;
	    MaxScore: number;
	    Grade: string;
	    PassCount: number;
	    FailCount: number;
	    Deductions: Deduction[];
	
	    static createFrom(source: any = {}) {
	        return new YearScore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Year = source["Year"];
	        this.RawScore = source["RawScore"];
	        this.MaxScore = source["MaxScore"];
	        this.Grade = source["Grade"];
	        this.PassCount = source["PassCount"];
	        this.FailCount = source["FailCount"];
	        this.Deductions = this.convertValues(source["Deductions"], Deduction);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CalcStep {
	    desc: string;
	    expr: string;
	    value: number;
	
	    static createFrom(source: any = {}) {
	        return new CalcStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.desc = source["desc"];
	        this.expr = source["expr"];
	        this.value = source["value"];
	    }
	}
	export class InputValue {
	    source: string;
	    item: string;
	    year: string;
	    value: number;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new InputValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.item = source["item"];
	        this.year = source["year"];
	        this.value = source["value"];
	        this.note = source["note"];
	    }
	}
	export class CalcTrace {
	    indicator: string;
	    year: string;
	    formula: string;
	    inputs: Record<string, InputValue>;
	    steps: CalcStep[];
	    result: number;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new CalcTrace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.indicator = source["indicator"];
	        this.year = source["year"];
	        this.formula = source["formula"];
	        this.inputs = this.convertValues(source["inputs"], InputValue, true);
	        this.steps = this.convertValues(source["steps"], CalcStep);
	        this.result = source["result"];
	        this.note = source["note"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StepResult {
	    stepNum: number;
	    stepName: string;
	    yearlyData: Record<string, any>;
	    conclusion: string;
	    pass: Record<string, boolean>;
	    traces: CalcTrace[];
	
	    static createFrom(source: any = {}) {
	        return new StepResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stepNum = source["stepNum"];
	        this.stepName = source["stepName"];
	        this.yearlyData = source["yearlyData"];
	        this.conclusion = source["conclusion"];
	        this.pass = source["pass"];
	        this.traces = this.convertValues(source["traces"], CalcTrace);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AnalysisReport {
	    symbol: string;
	    companyName: string;
	    years: string[];
	    stepResults: StepResult[];
	    passSummary: Record<string, Array<PassItem>>;
	    score: Record<string, number>;
	    scoreDetails?: Record<string, YearScore>;
	    overallGrade: string;
	    markdownContent: string;
	    rim?: RIMData;
	    highlights: string[];
	    risks: string[];
	    riskAlert?: RiskAlertSummary;
	    qualityWarnings?: string[];
	    diff?: AnalysisDiff;
	    quarterlyAlert?: QuarterlyAlert;
	    ttmMetrics?: TTMMetrics;
	    interactQAs?: InteractQA[];
	
	    static createFrom(source: any = {}) {
	        return new AnalysisReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.companyName = source["companyName"];
	        this.years = source["years"];
	        this.stepResults = this.convertValues(source["stepResults"], StepResult);
	        this.passSummary = this.convertValues(source["passSummary"], Array<PassItem>, true);
	        this.score = source["score"];
	        this.scoreDetails = this.convertValues(source["scoreDetails"], YearScore, true);
	        this.overallGrade = source["overallGrade"];
	        this.markdownContent = source["markdownContent"];
	        this.rim = this.convertValues(source["rim"], RIMData);
	        this.highlights = source["highlights"];
	        this.risks = source["risks"];
	        this.riskAlert = this.convertValues(source["riskAlert"], RiskAlertSummary);
	        this.qualityWarnings = source["qualityWarnings"];
	        this.diff = this.convertValues(source["diff"], AnalysisDiff);
	        this.quarterlyAlert = this.convertValues(source["quarterlyAlert"], QuarterlyAlert);
	        this.ttmMetrics = this.convertValues(source["ttmMetrics"], TTMMetrics);
	        this.interactQAs = this.convertValues(source["interactQAs"], InteractQA);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class ComparableRecommendation {
	    symbol: string;
	    name: string;
	    score: number;
	    reasons: string[];
	    dataQuality: string;
	
	    static createFrom(source: any = {}) {
	        return new ComparableRecommendation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.score = source["score"];
	        this.reasons = source["reasons"];
	        this.dataQuality = source["dataQuality"];
	    }
	}
	
	export class IndustryMetrics {
	    industry: string;
	    count: number;
	    roe: number;
	    roe_median: number;
	    gross_margin: number;
	    revenue_growth: number;
	    debt_ratio: number;
	    cash_ratio: number;
	    m_score: number;
	    inventory_turnover: number;
	    receivable_ratio: number;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new IndustryMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.industry = source["industry"];
	        this.count = source["count"];
	        this.roe = source["roe"];
	        this.roe_median = source["roe_median"];
	        this.gross_margin = source["gross_margin"];
	        this.revenue_growth = source["revenue_growth"];
	        this.debt_ratio = source["debt_ratio"];
	        this.cash_ratio = source["cash_ratio"];
	        this.m_score = source["m_score"];
	        this.inventory_turnover = source["inventory_turnover"];
	        this.receivable_ratio = source["receivable_ratio"];
	        this.updated_at = source["updated_at"];
	    }
	}
	
	
	
	export class PassItem {
	    year: string;
	    passed: boolean;
	    value: any;
	
	    static createFrom(source: any = {}) {
	        return new PassItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.year = source["year"];
	        this.passed = source["passed"];
	        this.value = source["value"];
	    }
	}
	
	
	
	
	
	
	
	
	
	
	export class RiskRadarItem {
	    name: string;
	    level: string;
	    status: string;
	    message: string;
	    icon: string;
	    value: string;
	    industry: string;
	    desc: string;
	
	    static createFrom(source: any = {}) {
	        return new RiskRadarItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.level = source["level"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.icon = source["icon"];
	        this.value = source["value"];
	        this.industry = source["industry"];
	        this.desc = source["desc"];
	    }
	}
	
	

}

export namespace downloader {
	
	export class ConceptConstituent {
	    code: string;
	    name: string;
	    market: string;
	    change_pct: number;
	    price: number;
	    main_inflow: number;
	    market_cap: number;
	    half_year_change_pct: number;
	
	    static createFrom(source: any = {}) {
	        return new ConceptConstituent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.market = source["market"];
	        this.change_pct = source["change_pct"];
	        this.price = source["price"];
	        this.main_inflow = source["main_inflow"];
	        this.market_cap = source["market_cap"];
	        this.half_year_change_pct = source["half_year_change_pct"];
	    }
	}
	export class HotConcept {
	    code: string;
	    name: string;
	    change_pct: number;
	    change_amt: number;
	    volume: number;
	    turnover: number;
	    main_inflow: number;
	    main_in_ratio: number;
	    top_stock: string;
	    top_stock_code: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new HotConcept(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.change_pct = source["change_pct"];
	        this.change_amt = source["change_amt"];
	        this.volume = source["volume"];
	        this.turnover = source["turnover"];
	        this.main_inflow = source["main_inflow"];
	        this.main_in_ratio = source["main_in_ratio"];
	        this.top_stock = source["top_stock"];
	        this.top_stock_code = source["top_stock_code"];
	        this.score = source["score"];
	    }
	}
	export class HotConceptBoard {
	    date: string;
	    updated_at: string;
	    concepts: HotConcept[];
	    data_source: string;
	    cache_version: number;
	
	    static createFrom(source: any = {}) {
	        return new HotConceptBoard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.updated_at = source["updated_at"];
	        this.concepts = this.convertValues(source["concepts"], HotConcept);
	        this.data_source = source["data_source"];
	        this.cache_version = source["cache_version"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HotConceptHistoryItem {
	    date: string;
	    top_names: string[];
	
	    static createFrom(source: any = {}) {
	        return new HotConceptHistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.top_names = source["top_names"];
	    }
	}
	export class IndustryUpdateResult {
	    success: boolean;
	    path: string;
	    total_industries: number;
	    updated_count: number;
	    skipped_count: number;
	    errors?: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new IndustryUpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.path = source["path"];
	        this.total_industries = source["total_industries"];
	        this.updated_count = source["updated_count"];
	        this.skipped_count = source["skipped_count"];
	        this.errors = source["errors"];
	        this.error = source["error"];
	    }
	}
	export class IntradayPoint {
	    time: string;
	    price: number;
	    avgPx: number;
	    volume: number;
	    amount: number;
	
	    static createFrom(source: any = {}) {
	        return new IntradayPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.price = source["price"];
	        this.avgPx = source["avgPx"];
	        this.volume = source["volume"];
	        this.amount = source["amount"];
	    }
	}
	export class IntradayData {
	    date: string;
	    prevClose: number;
	    points: IntradayPoint[];
	    isRealtime: boolean;
	    lastUpdated: string;
	
	    static createFrom(source: any = {}) {
	        return new IntradayData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.prevClose = source["prevClose"];
	        this.points = this.convertValues(source["points"], IntradayPoint);
	        this.isRealtime = source["isRealtime"];
	        this.lastUpdated = source["lastUpdated"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class KlineData {
	    time: string;
	    open: number;
	    close: number;
	    low: number;
	    high: number;
	    volume: number;
	    amount: number;
	    turnoverRate: number;
	
	    static createFrom(source: any = {}) {
	        return new KlineData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.open = source["open"];
	        this.close = source["close"];
	        this.low = source["low"];
	        this.high = source["high"];
	        this.volume = source["volume"];
	        this.amount = source["amount"];
	        this.turnoverRate = source["turnoverRate"];
	    }
	}
	export class PolicyUpdateResult {
	    success: boolean;
	    path: string;
	    added_industry_keywords: number;
	    added_concept_keywords: number;
	    total_industries: number;
	    total_concepts: number;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new PolicyUpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.path = source["path"];
	        this.added_industry_keywords = source["added_industry_keywords"];
	        this.added_concept_keywords = source["added_concept_keywords"];
	        this.total_industries = source["total_industries"];
	        this.total_concepts = source["total_concepts"];
	        this.errors = source["errors"];
	    }
	}
	export class StockConcepts {
	    concepts: string[];
	    wind: string;
	
	    static createFrom(source: any = {}) {
	        return new StockConcepts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.concepts = source["concepts"];
	        this.wind = source["wind"];
	    }
	}
	export class StockQuote {
	    currentPrice: number;
	    changePercent: number;
	    changeAmount: number;
	    volume: number;
	    turnoverAmount: number;
	    turnoverRate: number;
	    amplitude: number;
	    high: number;
	    low: number;
	    open: number;
	    previousClose: number;
	    circulatingMarketCap: number;
	    volumeRatio: number;
	    pe: number;
	    pb: number;
	    dividendYield: number;
	    shareholderReturnRate: number;
	    marketCap: number;
	    quoteTime: string;
	
	    static createFrom(source: any = {}) {
	        return new StockQuote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentPrice = source["currentPrice"];
	        this.changePercent = source["changePercent"];
	        this.changeAmount = source["changeAmount"];
	        this.volume = source["volume"];
	        this.turnoverAmount = source["turnoverAmount"];
	        this.turnoverRate = source["turnoverRate"];
	        this.amplitude = source["amplitude"];
	        this.high = source["high"];
	        this.low = source["low"];
	        this.open = source["open"];
	        this.previousClose = source["previousClose"];
	        this.circulatingMarketCap = source["circulatingMarketCap"];
	        this.volumeRatio = source["volumeRatio"];
	        this.pe = source["pe"];
	        this.pb = source["pb"];
	        this.dividendYield = source["dividendYield"];
	        this.shareholderReturnRate = source["shareholderReturnRate"];
	        this.marketCap = source["marketCap"];
	        this.quoteTime = source["quoteTime"];
	    }
	}
	export class ValidationResult {
	    year: string;
	    indicator: string;
	    hsf10Value: number;
	    dcValue: number;
	    diffPercent: number;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new ValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.year = source["year"];
	        this.indicator = source["indicator"];
	        this.hsf10Value = source["hsf10Value"];
	        this.dcValue = source["dcValue"];
	        this.diffPercent = source["diffPercent"];
	        this.status = source["status"];
	    }
	}

}

export namespace main {
	
	export class CacheStatus {
	    unchanged: boolean;
	    lastAnalysisAt: string;
	    dataChanged: boolean;
	    comparablesChanged: boolean;
	    dataMissing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CacheStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.unchanged = source["unchanged"];
	        this.lastAnalysisAt = source["lastAnalysisAt"];
	        this.dataChanged = source["dataChanged"];
	        this.comparablesChanged = source["comparablesChanged"];
	        this.dataMissing = source["dataMissing"];
	    }
	}
	export class DownloadResult {
	    success: boolean;
	    message: string;
	    years: string[];
	    quarters: string[];
	    validation: downloader.ValidationResult[];
	    sourceName: string;
	    qualityScore: number;
	    sourceSuggestion: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.years = source["years"];
	        this.quarters = source["quarters"];
	        this.validation = this.convertValues(source["validation"], downloader.ValidationResult);
	        this.sourceName = source["sourceName"];
	        this.qualityScore = source["qualityScore"];
	        this.sourceSuggestion = source["sourceSuggestion"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FetchMissingActivityResult {
	    successCount: number;
	    failCount: number;
	    failedCodes: string[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new FetchMissingActivityResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.successCount = source["successCount"];
	        this.failCount = source["failCount"];
	        this.failedCodes = source["failedCodes"];
	        this.message = source["message"];
	    }
	}
	export class FinancialTrendItem {
	    year: string;
	    roe?: number;
	    grossMargin?: number;
	    revenueGrowth?: number;
	    cashContent?: number;
	    debtRatio?: number;
	
	    static createFrom(source: any = {}) {
	        return new FinancialTrendItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.year = source["year"];
	        this.roe = source["roe"];
	        this.grossMargin = source["grossMargin"];
	        this.revenueGrowth = source["revenueGrowth"];
	        this.cashContent = source["cashContent"];
	        this.debtRatio = source["debtRatio"];
	    }
	}
	export class FinancialTrendsData {
	    symbol: string;
	    items: FinancialTrendItem[];
	
	    static createFrom(source: any = {}) {
	        return new FinancialTrendsData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.items = this.convertValues(source["items"], FinancialTrendItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HistoryMeta {
	    timestamp: string;
	    source: string;
	    sourceName: string;
	    years: string[];
	
	    static createFrom(source: any = {}) {
	        return new HistoryMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.source = source["source"];
	        this.sourceName = source["sourceName"];
	        this.years = source["years"];
	    }
	}
	export class ImportResult {
	    success: boolean;
	    message: string;
	    balanceSheet: string[];
	    income: string[];
	    cashFlow: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.balanceSheet = source["balanceSheet"];
	        this.income = source["income"];
	        this.cashFlow = source["cashFlow"];
	    }
	}
	export class PythonPackage {
	    name: string;
	    moduleName: string;
	    display: string;
	    required: boolean;
	    installed: boolean;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new PythonPackage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.moduleName = source["moduleName"];
	        this.display = source["display"];
	        this.required = source["required"];
	        this.installed = source["installed"];
	        this.version = source["version"];
	    }
	}
	export class PythonEnvResult {
	    pythonFound: boolean;
	    pythonPath: string;
	    version: string;
	    packages: PythonPackage[];
	    allReady: boolean;
	    ready: boolean;
	    missing: string[];
	
	    static createFrom(source: any = {}) {
	        return new PythonEnvResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pythonFound = source["pythonFound"];
	        this.pythonPath = source["pythonPath"];
	        this.version = source["version"];
	        this.packages = this.convertValues(source["packages"], PythonPackage);
	        this.allReady = source["allReady"];
	        this.ready = source["ready"];
	        this.missing = source["missing"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class QuickAnalysis {
	    code: string;
	    name: string;
	    symbol: string;
	    market: string;
	    current_price: number;
	    change_percent: number;
	    turnover_rate: number;
	    volume_ratio: number;
	    has_moneyflow_data: boolean;
	    main_inflow: number;
	    sm_net_amount: number;
	    md_net_amount: number;
	    lg_net_amount: number;
	    elg_net_amount: number;
	    industry: string;
	    market_cap: number;
	    pe: number;
	    pb: number;
	    eps: number;
	    sentiment_score: number;
	    sentiment_heat: number;
	    sentiment_keywords: string[];
	    has_sentiment_data: boolean;
	    concepts: string[];
	    concept_match: string[];
	    riskAlert?: analyzer.RiskAlertSummary;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new QuickAnalysis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.symbol = source["symbol"];
	        this.market = source["market"];
	        this.current_price = source["current_price"];
	        this.change_percent = source["change_percent"];
	        this.turnover_rate = source["turnover_rate"];
	        this.volume_ratio = source["volume_ratio"];
	        this.has_moneyflow_data = source["has_moneyflow_data"];
	        this.main_inflow = source["main_inflow"];
	        this.sm_net_amount = source["sm_net_amount"];
	        this.md_net_amount = source["md_net_amount"];
	        this.lg_net_amount = source["lg_net_amount"];
	        this.elg_net_amount = source["elg_net_amount"];
	        this.industry = source["industry"];
	        this.market_cap = source["market_cap"];
	        this.pe = source["pe"];
	        this.pb = source["pb"];
	        this.eps = source["eps"];
	        this.sentiment_score = source["sentiment_score"];
	        this.sentiment_heat = source["sentiment_heat"];
	        this.sentiment_keywords = source["sentiment_keywords"];
	        this.has_sentiment_data = source["has_sentiment_data"];
	        this.concepts = source["concepts"];
	        this.concept_match = source["concept_match"];
	        this.riskAlert = this.convertValues(source["riskAlert"], analyzer.RiskAlertSummary);
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SFLConfig {
	    enabled: boolean;
	    token: string;
	    verified: boolean;
	    verified_at: string;
	    use_for_financial: boolean;
	    use_for_kline: boolean;
	    use_for_quote: boolean;
	    use_for_moneyflow: boolean;
	    moneyflow_days: number;
	
	    static createFrom(source: any = {}) {
	        return new SFLConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.token = source["token"];
	        this.verified = source["verified"];
	        this.verified_at = source["verified_at"];
	        this.use_for_financial = source["use_for_financial"];
	        this.use_for_kline = source["use_for_kline"];
	        this.use_for_quote = source["use_for_quote"];
	        this.use_for_moneyflow = source["use_for_moneyflow"];
	        this.moneyflow_days = source["moneyflow_days"];
	    }
	}
	export class SFLVerifyResult {
	    success: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new SFLVerifyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	    }
	}
	export class SnapshotInfo {
	    timestamp: string;
	    date_time: string;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.date_time = source["date_time"];
	    }
	}
	export class StockInfo {
	    code: string;
	    name: string;
	    market: string;
	
	    static createFrom(source: any = {}) {
	        return new StockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.market = source["market"];
	    }
	}
	export class StockMoneyflowItem {
	    date: string;
	    main_inflow: number;
	    sm_net_amount: number;
	    md_net_amount: number;
	    lg_net_amount: number;
	    elg_net_amount: number;
	
	    static createFrom(source: any = {}) {
	        return new StockMoneyflowItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.main_inflow = source["main_inflow"];
	        this.sm_net_amount = source["sm_net_amount"];
	        this.md_net_amount = source["md_net_amount"];
	        this.lg_net_amount = source["lg_net_amount"];
	        this.elg_net_amount = source["elg_net_amount"];
	    }
	}
	export class StockMoneyflowResult {
	    symbol: string;
	    items: StockMoneyflowItem[];
	    has_data: boolean;
	    summary: string;
	    days: number;
	    today_item?: StockMoneyflowItem;
	
	    static createFrom(source: any = {}) {
	        return new StockMoneyflowResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.items = this.convertValues(source["items"], StockMoneyflowItem);
	        this.has_data = source["has_data"];
	        this.summary = source["summary"];
	        this.days = source["days"];
	        this.today_item = this.convertValues(source["today_item"], StockMoneyflowItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockProfile {
	    industry: string;
	    listingDate: string;
	    totalShares: number;
	    marketCap: number;
	    pe: number;
	    pb: number;
	    eps: number;
	    chairman: string;
	    controller: string;
	    chairmanGender: string;
	    chairmanAge: string;
	    chairmanNationality: string;
	    chairmanHoldRatio: string;
	    politicalAffiliation: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new StockProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.industry = source["industry"];
	        this.listingDate = source["listingDate"];
	        this.totalShares = source["totalShares"];
	        this.marketCap = source["marketCap"];
	        this.pe = source["pe"];
	        this.pb = source["pb"];
	        this.eps = source["eps"];
	        this.chairman = source["chairman"];
	        this.controller = source["controller"];
	        this.chairmanGender = source["chairmanGender"];
	        this.chairmanAge = source["chairmanAge"];
	        this.chairmanNationality = source["chairmanNationality"];
	        this.chairmanHoldRatio = source["chairmanHoldRatio"];
	        this.politicalAffiliation = source["politicalAffiliation"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class WatchlistActivitySummary {
	    code: string;
	    score: number;
	    stars: number;
	    grade: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistActivitySummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.score = source["score"];
	        this.stars = source["stars"];
	        this.grade = source["grade"];
	    }
	}
	export class WatchlistFilterItem {
	    code: string;
	    industry: string;
	    shareholderReturnRate: number;
	    aScore: number;
	    riskLevel: string;
	    hasFinancialData: boolean;
	    hasSnapshot: boolean;
	    lastAnalyzedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistFilterItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.industry = source["industry"];
	        this.shareholderReturnRate = source["shareholderReturnRate"];
	        this.aScore = source["aScore"];
	        this.riskLevel = source["riskLevel"];
	        this.hasFinancialData = source["hasFinancialData"];
	        this.hasSnapshot = source["hasSnapshot"];
	        this.lastAnalyzedAt = source["lastAnalyzedAt"];
	    }
	}
	export class WatchlistItem {
	    code: string;
	    name: string;
	    market: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.market = source["market"];
	    }
	}

}

export namespace updater {
	
	export class UpdateInfo {
	    hasUpdate: boolean;
	    currentVer: string;
	    latestVer: string;
	    releaseName: string;
	    releaseNote: string;
	    publishedAt: string;
	    assetURL: string;
	    htmlURL: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasUpdate = source["hasUpdate"];
	        this.currentVer = source["currentVer"];
	        this.latestVer = source["latestVer"];
	        this.releaseName = source["releaseName"];
	        this.releaseNote = source["releaseNote"];
	        this.publishedAt = source["publishedAt"];
	        this.assetURL = source["assetURL"];
	        this.htmlURL = source["htmlURL"];
	    }
	}

}

