package ag.sidiora.mobile
import com.facebook.react.ReactActivity
import com.facebook.react.ReactActivityDelegate
import com.facebook.react.defaults.DefaultNewArchitectureEntryPoint.fabricEnabled
import com.facebook.react.defaults.DefaultReactActivityDelegate
class MainActivity : ReactActivity() {
 override fun onCreate(savedInstanceState:android.os.Bundle?){window.addFlags(android.view.WindowManager.LayoutParams.FLAG_SECURE);applyNotificationLink(intent);super.onCreate(savedInstanceState)}
 private fun applyNotificationLink(value:android.content.Intent?){val link=value?.getStringExtra("deepLink")?:return;if(link.length<=2048){value.action=android.content.Intent.ACTION_VIEW;value.data=android.net.Uri.parse(link);value.removeExtra("deepLink")}}
 override fun onNewIntent(intent:android.content.Intent){applyNotificationLink(intent);setIntent(intent);super.onNewIntent(intent)}
 override fun onPause(){window.addFlags(android.view.WindowManager.LayoutParams.FLAG_SECURE);super.onPause()}
 override fun getMainComponentName() = "Sidiora"
 override fun createReactActivityDelegate(): ReactActivityDelegate = DefaultReactActivityDelegate(this, mainComponentName, fabricEnabled)
}
