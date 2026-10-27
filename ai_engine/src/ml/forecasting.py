import pandas as pd
import numpy as np
from xgboost import XGBRegressor

class InventoryForecaster:
    def __init__(self):
        self.model = XGBRegressor(n_estimators=100, max_depth=5, learning_rate=0.05)

    def _generate_future_features(self, df: pd.DataFrame, days: int = 30) -> pd.DataFrame:
        last_date = df['date'].max()
        future_dates = pd.date_range(start=last_date + pd.Timedelta(days=1), periods=days)
        
        # Simplified feature generation for the future based on recent mean bounds.
        # In actual production, you'd carry forward the rolling means contextually.
        future_df = pd.DataFrame({'date': future_dates})
        future_df['dayofweek'] = future_df['date'].dt.dayofweek
        
        last_rolling_mean = df['rolling_mean_7d'].iloc[-1] if not df.empty else 0
        future_df['rolling_mean_7d'] = last_rolling_mean 
        
        return future_df[['dayofweek', 'rolling_mean_7d']]

    def predict_depletion_days(self, sales_history_df: pd.DataFrame, current_stock: float) -> int:
        """
        Predicts how many days until stock hits zero based on seasonality, day-of-week, and trend.
        """
        if sales_history_df.empty or len(sales_history_df) < 7:
            return 30 # Fallback

        sales_history_df['dayofweek'] = sales_history_df['date'].dt.dayofweek
        sales_history_df['rolling_mean_7d'] = sales_history_df['qty_sold'].rolling(7).mean()
        
        # Prepare features X and target y
        X = sales_history_df[['dayofweek', 'rolling_mean_7d']].dropna()
        if len(X) == 0:
            return 30
            
        y = sales_history_df['qty_sold'].iloc[len(sales_history_df)-len(X):]
        
        self.model.fit(X, y)
        
        future_features = self._generate_future_features(sales_history_df, days=30)
        predicted_daily_sales = self.model.predict(future_features)
        
        cumulative_sales = np.cumsum(predicted_daily_sales)
        # Find where cumulative sales match or exceed current stock
        indices = np.where(cumulative_sales >= current_stock)[0]
        
        if len(indices) > 0:
            return int(indices[0])
        return 30
