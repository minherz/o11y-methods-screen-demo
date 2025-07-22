package com.example.demo;

import com.google.api.client.util.Strings;
import com.google.cloud.MetadataConfig;

public class Metadata {

    private static String projectId, region;

    public static String projectId() {
        if (!Strings.isNullOrEmpty(projectId)) {
            return projectId;
        }
        projectId = MetadataConfig.getProjectId();
        return projectId;
    }

    public static String region() {
        if (!Strings.isNullOrEmpty(region)) {
            return region;
        }
        region = MetadataConfig.getAttribute("instance/region");
        if (region == null) {
            region = "us-west1";
        }
        int idx = region.lastIndexOf("/");
        if (idx >= 0) {
            region = region.substring(idx + 1);
        }
        return region;
    }
}
