package com.example;

import java.time.Instant;
import java.util.Map;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class HeavyController {

    @Value("${heavy.calc.n:40}")
    private int heavyCalcN;

    private static long fibonacci(int n) {
        if (n <= 1) return n;
        return fibonacci(n - 1) + fibonacci(n - 2);
    }

    @GetMapping("/health")
    public Map<String, String> health() {
        return Map.of("status", "ok", "language", "java");
    }

    @GetMapping("/heavy")
    public Map<String, Object> heavy(@RequestParam(value = "n", required = false) Integer n) {
        int calcN = (n != null) ? n : heavyCalcN;
        Instant startedAt = Instant.now();
        long startMs = System.currentTimeMillis();

        fibonacci(calcN);

        long endMs = System.currentTimeMillis();
        Instant finishedAt = Instant.now();

        return Map.of(
            "language", "java",
            "threadId", Thread.currentThread().getName() + "-" + Thread.currentThread().getId(),
            "startedAt", startedAt.toString(),
            "finishedAt", finishedAt.toString(),
            "durationMs", endMs - startMs
        );
    }
}
