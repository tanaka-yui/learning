<?php

use App\Http\Controllers\HeavyController;
use Illuminate\Support\Facades\Route;

Route::get('/health', [HeavyController::class, 'health']);
Route::get('/heavy', [HeavyController::class, 'heavy']);
