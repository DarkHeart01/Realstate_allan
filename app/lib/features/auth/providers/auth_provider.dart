import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../../../core/network/dio_client.dart';

const _kAccessTokenKey = 'access_token';

class AuthState {
  final bool isLoggedIn;
  final bool isLoading;
  final String? error;

  const AuthState({
    this.isLoggedIn = false,
    this.isLoading = false,
    this.error,
  });

  AuthState copyWith({bool? isLoggedIn, bool? isLoading, String? error}) =>
      AuthState(
        isLoggedIn: isLoggedIn ?? this.isLoggedIn,
        isLoading: isLoading ?? this.isLoading,
        error: error,
      );
}

class AuthNotifier extends StateNotifier<AuthState> {
  AuthNotifier(this._dio) : super(const AuthState()) {
    _checkToken();
  }

  final Dio _dio;
  final _storage = const FlutterSecureStorage();

  Future<void> _checkToken() async {
    final token = await _storage.read(key: _kAccessTokenKey);
    state = state.copyWith(isLoggedIn: token != null && token.isNotEmpty);
  }

  Future<void> login(String email, String password) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final res = await _dio.post('/api/auth/login', data: {
        'email': email,
        'password': password,
      });
      final token = res.data['data']['access_token'] as String?;
      if (token == null || token.isEmpty) throw Exception('No token received');
      await _storage.write(key: _kAccessTokenKey, value: token);
      state = state.copyWith(isLoggedIn: true, isLoading: false);
    } on DioException catch (e) {
      final msg = e.response?.data?['message'] ?? 'Login failed';
      state = state.copyWith(isLoading: false, error: msg.toString());
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  void logout() {
    state = const AuthState(); // sync — triggers router redirect immediately
    _storage.delete(key: _kAccessTokenKey); // background cleanup, no need to await
  }
}

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  return AuthNotifier(createDioClient());
});
