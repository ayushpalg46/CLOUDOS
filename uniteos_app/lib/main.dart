import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:google_fonts/google_fonts.dart';

import 'providers/sync_provider.dart';
import 'screens/home_screen.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  SystemChrome.setSystemUIOverlayStyle(
    const SystemUiOverlayStyle(
      statusBarColor: Colors.transparent,
      statusBarIconBrightness: Brightness.light,
    ),
  );
  runApp(
    MultiProvider(
      providers: [
        ChangeNotifierProvider(create: (_) => SyncProvider()),
      ],
      child: const uniteOSApp(),
    ),
  );
}

// ─── Design Tokens ───────────────────────────────────────────────────────────
class AppColors {
  static const bg           = Color(0xFF0E0E0E);
  static const surface      = Color(0xFF1A1A1A);
  static const border       = Color(0xFF2E2E2E);
  static const yellow       = Color(0xFFF5C518);
  static const yellowDim    = Color(0xFF3D3000);
  static const red          = Color(0xFFE53935);
  static const blue         = Color(0xFF1565C0);
  static const green        = Color(0xFF2E7D32);
  static const textPrimary  = Color(0xFFEEEEEE);
  static const textSecondary= Color(0xFF888888);
  static const textMuted    = Color(0xFF555555);
}

class uniteOSApp extends StatelessWidget {
  const uniteOSApp({super.key});

  @override
  Widget build(BuildContext context) {
    final mono = GoogleFonts.jetBrainsMono();
    return MaterialApp(
      title: 'uniteOS',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        useMaterial3: false,
        scaffoldBackgroundColor: AppColors.bg,
        colorScheme: const ColorScheme.dark(
          surface: AppColors.surface,
          primary: AppColors.yellow,
          secondary: AppColors.yellow,
          onPrimary: Colors.black,
        ),
        textTheme: GoogleFonts.jetBrainsMonoTextTheme(
          ThemeData.dark().textTheme.apply(
            bodyColor: AppColors.textPrimary,
            displayColor: AppColors.textPrimary,
          ),
        ),
        dividerColor: AppColors.border,
        cardColor: AppColors.surface,
        iconTheme: const IconThemeData(color: AppColors.textPrimary),
        appBarTheme: AppBarTheme(
          backgroundColor: AppColors.bg,
          foregroundColor: AppColors.textPrimary,
          elevation: 0,
          systemOverlayStyle: SystemUiOverlayStyle.light,
          titleTextStyle: mono.copyWith(
            color: AppColors.textPrimary,
            fontSize: 14,
            fontWeight: FontWeight.w700,
            letterSpacing: 2,
          ),
        ),
      ),
      home: const HomeScreen(),
    );
  }
}
