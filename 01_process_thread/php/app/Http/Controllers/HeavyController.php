<?php

namespace App\Http\Controllers;

use Illuminate\Http\JsonResponse;

class HeavyController extends Controller
{
    private int $heavyCalcN;

    public function __construct()
    {
        $envValue = getenv('HEAVY_CALC_N');
        $this->heavyCalcN = $envValue !== false ? (int) $envValue : 40;
    }

    private function fibonacci(int $n): int
    {
        if ($n <= 1) {
            return $n;
        }
        return $this->fibonacci($n - 1) + $this->fibonacci($n - 2);
    }

    public function health(): JsonResponse
    {
        return response()->json([
            'status' => 'ok',
            'language' => 'php',
        ]);
    }

    public function heavy(): JsonResponse
    {
        $startedAt = gmdate('Y-m-d\TH:i:s.v\Z');
        $startMs = hrtime(true);

        $this->fibonacci($this->heavyCalcN);

        $endMs = hrtime(true);
        $finishedAt = gmdate('Y-m-d\TH:i:s.v\Z');

        $durationMs = (int) (($endMs - $startMs) / 1_000_000);

        return response()->json([
            'language' => 'php',
            'threadId' => 'fpm-worker-' . getmypid(),
            'startedAt' => $startedAt,
            'finishedAt' => $finishedAt,
            'durationMs' => $durationMs,
        ]);
    }
}
