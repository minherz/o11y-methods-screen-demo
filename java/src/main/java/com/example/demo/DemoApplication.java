package com.example.demo;

import io.opentelemetry.api.common.AttributeKey;
import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.api.OpenTelemetry;
import io.opentelemetry.api.metrics.LongCounter;

import java.io.IOException;
import java.util.Collections;

import javax.annotation.PostConstruct;
import javax.annotation.PreDestroy;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import com.google.cloud.ServiceOptions;
import com.google.cloud.MetadataConfig;
import com.google.cloud.vertexai.VertexAI;
import com.google.cloud.vertexai.api.GenerateContentResponse;
import com.google.cloud.vertexai.generativeai.GenerativeModel;
import com.google.cloud.vertexai.generativeai.ResponseHandler;

@SpringBootApplication
public class DemoApplication {

    public static void main(String[] args) {
        String port = System.getenv().getOrDefault("PORT", "8080");
        SpringApplication app = new SpringApplication(DemoApplication.class);
        app.setDefaultProperties(Collections.singletonMap("server.port", port));
        app.run(args);
    }
}

@RestController
class HelloController {
    private final String projectId = ServiceOptions.getDefaultProjectId();
    private VertexAI vertexAI;
    private GenerativeModel model;
    private final Logger LOGGER = LoggerFactory.getLogger(HelloController.class);
    private static final String INSTRUMENTATION_NAME = "o11y/demo/java";
    private static final Attributes LANGUAGE_ATTR = Attributes.of(AttributeKey.stringKey("language"), "java");
    private final LongCounter counter;

    public HelloController(OpenTelemetry openTelemetry) {
        this.counter = openTelemetry.getMeter(INSTRUMENTATION_NAME)
                .counterBuilder("model_call_counter")
                .setDescription("Number of successful model calls")
                .build();
    }

    @PostConstruct
    public void init() {
        vertexAI = new VertexAI(projectId, getRegion());
        model = new GenerativeModel(System.getenv().getOrDefault("MODEL_NAME", "gemini-2.5-flash"), vertexAI);
    }

    @PreDestroy
    public void destroy() {
        vertexAI.close();
    }

    @GetMapping("/facts")
    public String getFacts(@RequestParam(defaultValue = "dog") String subject) throws IOException {
        var prompt = "Give me 10 fun facts about " + subject + ". Return this as html without backticks.";
        var response = model.generateContent(prompt);
        LOGGER.atDebug()
                .addKeyValue("subject", subject)
                .addKeyValue("prompt", prompt)
                .addKeyValue("response", response)
                .log("Content is generated");
        counter.add(1, LANGUAGE_ATTR);
        return ResponseHandler.getText(response);
    }

	private String getRegion() {
		var region = MetadataConfig.getAttribute("instance/region");
		if (region == null) {
			return "us-west1";
		}
		int idx = region.lastIndexOf("/");
		if (idx >= 0) {
			return region.substring(idx+1);
		}
		return region;
	}
}